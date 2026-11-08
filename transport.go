package cotizave

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// errRedirectRefused is returned by the HTTP client's CheckRedirect so a
// redirect can never forward the X-API-Key header to another origin.
var errRedirectRefused = errors.New("cotizave: redirect refused")

// transport is the typed HTTP layer: authentication, origin pinning, redirect
// refusal, the retry policy, the response-size cap, and error mapping.
type transport struct {
	cfg        *config
	httpClient *http.Client
}

func newTransport(cfg *config, userClient *http.Client) *transport {
	var hc *http.Client
	if userClient != nil {
		clone := *userClient
		hc = &clone
	} else {
		hc = &http.Client{}
	}
	// We manage per-attempt deadlines via context, and refuse redirects so the
	// key is never forwarded off-origin — even on an injected client.
	hc.Timeout = 0
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return errRedirectRefused }
	return &transport{cfg: cfg, httpClient: hc}
}

// requestSpec is an internal, fully-specified request.
type requestSpec struct {
	method      string
	path        string
	query       url.Values
	body        any
	cfg         *RequestConfig
	accept      string
	ifNoneMatch string
	maxBytes    int64
}

// rawResponse is a successful (non-error) HTTP outcome.
type rawResponse struct {
	body        []byte
	meta        ResponseMeta
	notModified bool
}

// httpOutcome is the result of a single attempt that received an HTTP response.
type httpOutcome struct {
	meta        ResponseMeta
	body        []byte
	notModified bool
	apiErr      error // non-nil for a >=400 response
}

// RequestConfig holds per-call overrides accepted by every resource method.
type RequestConfig struct {
	// Timeout overrides the client-wide timeout for this call.
	Timeout time.Duration
	// MaxRetries overrides the client-wide MaxRetries for this call. A negative
	// value disables retries.
	MaxRetries *int
	// Headers are extra headers for this call; they override the client-wide
	// Headers. They cannot set X-API-Key.
	Headers map[string]string
	// Language overrides the client-wide Accept-Language for this call.
	Language string
}

func firstConfig(cfgs []RequestConfig) *RequestConfig {
	if len(cfgs) == 0 {
		return nil
	}
	return &cfgs[0]
}

// do runs a request with the retry policy, bounded by MaxRetries and the
// MaxRetryDelay budget.
func (t *transport) do(ctx context.Context, spec requestSpec) (*rawResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	method := spec.method
	if method == "" {
		method = http.MethodGet
	}

	fullURL, err := t.buildURL(spec.path, spec.query)
	if err != nil {
		return nil, err
	}
	header, err := t.buildHeaders(method, spec)
	if err != nil {
		return nil, err
	}

	timeout := t.cfg.timeout
	maxRetries := t.cfg.maxRetries
	if spec.cfg != nil {
		if spec.cfg.Timeout > 0 {
			timeout = spec.cfg.Timeout
		}
		if spec.cfg.MaxRetries != nil {
			if mr := *spec.cfg.MaxRetries; mr >= 0 {
				maxRetries = mr
			} else {
				maxRetries = 0
			}
		}
	}
	maxBytes := t.cfg.maxResponseBytes
	if spec.maxBytes > 0 {
		maxBytes = spec.maxBytes
	}

	var bodyBytes []byte
	if method == http.MethodPost && spec.body != nil {
		bodyBytes, err = json.Marshal(spec.body)
		if err != nil {
			return nil, &Error{Code: "invalid_request_body", Message: "could not encode the request body as JSON", cause: err}
		}
	}

	budget := t.cfg.maxRetryDelay
	spent := time.Duration(0)
	var lastErr error

	for attempt := 0; ; attempt++ {
		outcome, transportErr := t.attempt(ctx, method, spec.path, fullURL, header, bodyBytes, timeout, maxBytes)
		if transportErr != nil {
			lastErr = transportErr
		} else if outcome.apiErr != nil {
			lastErr = outcome.apiErr
		} else {
			return &rawResponse{body: outcome.body, meta: outcome.meta, notModified: outcome.notModified}, nil
		}

		if attempt >= maxRetries || !shouldRetry(t.cfg.retry, method, spec.path, lastErr, attempt) {
			return nil, lastErr
		}
		delay := nextDelay(t.cfg.retry, lastErr, attempt, budget-spent)
		if delay < 0 {
			return nil, lastErr
		}
		if t.cfg.retry.onRetry != nil {
			fireOnRetry(t.cfg.retry.onRetry, RetryEvent{
				Err:         lastErr,
				RetryNumber: attempt + 1,
				Delay:       delay,
				Method:      method,
				Path:        spec.path,
			})
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, &Error{Code: "aborted", Message: "request canceled by caller", cause: ctx.Err()}
		case <-timer.C:
		}
		spent += delay
	}
}

func (t *transport) attempt(
	ctx context.Context,
	method, path, fullURL string,
	header http.Header,
	body []byte,
	timeout time.Duration,
	maxBytes int64,
) (*httpOutcome, error) {
	reqCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(reqCtx, method, fullURL, bodyReader)
	if err != nil {
		return nil, &Error{Code: "invalid_request", Message: err.Error(), cause: err}
	}
	req.Header = header.Clone()

	resp, err := t.httpClient.Do(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return nil, t.mapTransportError(err, fullURL, timeout, reqCtx, ctx)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxBytes+1)
	data, readErr := io.ReadAll(limited)
	if readErr != nil {
		return nil, t.mapTransportError(readErr, fullURL, timeout, reqCtx, ctx)
	}
	if int64(len(data)) > maxBytes {
		return nil, &Error{
			Code:    "response_too_large",
			Message: fmt.Sprintf("response body exceeded the maximum allowed size (%d bytes)", maxBytes),
		}
	}

	meta := buildMeta(resp.StatusCode, resp.Header)
	t.fireOnResponse(method, path, meta)

	if resp.StatusCode == http.StatusNotModified {
		return &httpOutcome{meta: meta, notModified: true}, nil
	}
	if resp.StatusCode >= 400 {
		return &httpOutcome{meta: meta, apiErr: errorFromResponse(resp.StatusCode, data, meta)}, nil
	}
	return &httpOutcome{meta: meta, body: data}, nil
}

func (t *transport) mapTransportError(err error, fullURL string, timeout time.Duration, reqCtx, parentCtx context.Context) *Error {
	if errors.Is(err, errRedirectRefused) {
		return &Error{
			Code:         "unexpected_redirect",
			Message:      "refusing to follow a redirect; the API is not expected to redirect and this could forward the API key off-origin",
			isConnection: false,
			cause:        err,
		}
	}
	if parentCtx.Err() != nil {
		return &Error{Code: "aborted", Message: "request canceled by caller", cause: err}
	}
	if errors.Is(err, context.DeadlineExceeded) || reqCtx.Err() == context.DeadlineExceeded {
		return &Error{
			Code:         "timeout",
			Message:      fmt.Sprintf("request to %s timed out after %s", fullURL, timeout),
			Timeout:      true,
			isConnection: true,
			cause:        err,
		}
	}
	return &Error{
		Code:         "connection_error",
		Message:      fmt.Sprintf("network error while requesting %s: %s", fullURL, err.Error()),
		isConnection: true,
		cause:        err,
	}
}

// buildURL joins the base and path, pins the result to the configured origin,
// and applies the query. A path that resolves off-origin is refused so the
// X-API-Key header can never be sent elsewhere.
func (t *transport) buildURL(path string, query url.Values) (string, error) {
	u, err := url.Parse(t.cfg.baseURL + path)
	if err != nil {
		return "", &Error{Code: "invalid_request_path", Message: fmt.Sprintf("invalid request path %q", path), cause: err}
	}
	if origin := u.Scheme + "://" + u.Host; origin != t.cfg.origin {
		return "", &Error{
			Code:    "invalid_request_path",
			Message: fmt.Sprintf("refusing to send a request to a different origin (%s); a request path must stay on %s", origin, t.cfg.origin),
		}
	}
	if len(query) > 0 {
		q := u.Query()
		for k, vs := range query {
			if len(vs) > 0 {
				q.Set(k, vs[0])
			}
		}
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

// buildHeaders applies, lowest to highest precedence: SDK defaults, client-wide
// headers and language, per-call headers and language, Accept / If-None-Match,
// and finally the API key, which nothing can override.
func (t *transport) buildHeaders(method string, spec requestSpec) (http.Header, error) {
	h := http.Header{}
	h.Set("Accept", "application/json")
	h.Set("User-Agent", t.cfg.userAgent)
	for k, v := range t.cfg.headers {
		h.Set(k, v)
	}
	if t.cfg.language != "" {
		h.Set("Accept-Language", t.cfg.language)
	}

	if spec.cfg != nil {
		perCall, err := validateHeaders(spec.cfg.Headers, "headers")
		if err != nil {
			return nil, &Error{Code: "invalid_header_value", Message: err.Error(), cause: err}
		}
		for k, v := range perCall {
			h.Set(k, v)
		}
		if spec.cfg.Language != "" {
			if err := validateHeaderValue(spec.cfg.Language, "language"); err != nil {
				return nil, &Error{Code: "invalid_header_value", Message: err.Error(), cause: err}
			}
			h.Set("Accept-Language", spec.cfg.Language)
		}
	}

	if spec.accept != "" {
		h.Set("Accept", spec.accept)
	}
	if spec.ifNoneMatch != "" {
		if err := validateHeaderValue(spec.ifNoneMatch, "ifNoneMatch"); err != nil {
			return nil, &Error{Code: "invalid_header_value", Message: err.Error(), cause: err}
		}
		h.Set("If-None-Match", spec.ifNoneMatch)
	}
	if method == http.MethodPost && spec.body != nil {
		h.Set("Content-Type", "application/json")
	}
	// The API key goes last and is owned by the SDK: nothing can override it.
	h.Set("X-API-Key", t.cfg.apiKey)
	return h, nil
}

func (t *transport) fireOnResponse(method, path string, meta ResponseMeta) {
	if t.cfg.onResponse == nil {
		return
	}
	defer func() { _ = recover() }() // an observability hook must never break a request
	t.cfg.onResponse(ResponseEvent{Method: method, Path: path, Meta: meta})
}

func fireOnRetry(hook func(RetryEvent), ev RetryEvent) {
	defer func() { _ = recover() }()
	hook(ev)
}

// buildMeta builds response metadata (status, headers, request id, rate limit).
func buildMeta(status int, h http.Header) ResponseMeta {
	headers := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			headers[strings.ToLower(k)] = v[0]
		}
	}
	meta := ResponseMeta{Status: status, Headers: headers, RateLimit: parseRateLimit(h)}
	if id := h.Get("X-Request-Id"); id != "" {
		meta.RequestID = id
	}
	return meta
}

func parseRateLimit(h http.Header) RateLimit {
	rl := RateLimit{}
	if s := h.Get("X-RateLimit-Scope"); s == "monthly" || s == "burst" {
		rl.Scope = s
	}
	if n, ok := headerInt(h, "X-RateLimit-Limit"); ok {
		rl.Limit, rl.hasLimit = n, true
	}
	if n, ok := headerInt(h, "X-RateLimit-Remaining"); ok {
		rl.Remaining, rl.hasRemaining = n, true
	}
	if v := h.Get("X-RateLimit-Reset"); v != "" {
		rl.Reset = v
	}
	if n, ok := headerInt(h, "X-RateLimit-Limit-Minute"); ok {
		rl.LimitPerMinute, rl.hasLimitMin = n, true
	}
	if n, ok := headerInt(h, "X-RateLimit-Remaining-Minute"); ok {
		rl.RemainingPerMinute, rl.hasRemainMin = n, true
	}
	if n, ok := headerInt(h, "Retry-After"); ok {
		rl.RetryAfter, rl.hasRetryAfter = n, true
	}
	if n, ok := headerInt(h, "X-RateLimit-Retry-After-Ms"); ok {
		rl.RetryAfterMs, rl.hasRetryMs = n, true
	}
	return rl
}

func headerInt(h http.Header, key string) (int, bool) {
	raw := h.Get(key)
	if raw == "" {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, false
	}
	return n, true
}

func runtimeVersion() string {
	return strings.TrimPrefix(runtime.Version(), "go")
}

// --- Generic decode helpers used by the resource methods ---

func (t *transport) getJSON(ctx context.Context, path string, query url.Values, cfg *RequestConfig, out any) error {
	raw, err := t.do(ctx, requestSpec{method: http.MethodGet, path: path, query: query, cfg: cfg})
	if err != nil {
		return err
	}
	return decodeInto(raw, path, out)
}

func (t *transport) postJSON(ctx context.Context, path string, body any, cfg *RequestConfig, out any) error {
	raw, err := t.do(ctx, requestSpec{method: http.MethodPost, path: path, body: body, cfg: cfg})
	if err != nil {
		return err
	}
	return decodeInto(raw, path, out)
}

func decodeInto(raw *rawResponse, path string, out any) error {
	if raw.notModified || len(bytes.TrimSpace(raw.body)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw.body, out); err != nil {
		return &Error{
			Status:  raw.meta.Status,
			Code:    "invalid_json_response",
			Message: fmt.Sprintf("expected JSON from %s but could not parse the response body", path),
			Details: map[string]any{"non_json_body": truncate(string(raw.body), 500)},
			Meta:    &raw.meta,
			cause:   err,
		}
	}
	return nil
}
