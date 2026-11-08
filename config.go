package cotizave

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// Defaults applied when the corresponding Options field is left zero.
const (
	DefaultBaseURL          = "https://api.cotizave.com"
	DefaultTimeout          = 5 * time.Second
	DefaultHistoryTimeout   = 10 * time.Second
	DefaultMaxRetries       = 2
	DefaultMaxRetryDelay    = 10 * time.Second
	DefaultMaxResponseBytes = 25_000_000
	DefaultRetryBaseDelay   = 250 * time.Millisecond
	DefaultRetryMaxDelay    = 2 * time.Second

	// apiKeyEnvVar is read when Options.APIKey is empty.
	apiKeyEnvVar = "COTIZAVE_API_KEY"
	reservedHdr  = "x-api-key"
)

// defaultRetryStatuses are the HTTP statuses retried for idempotent (GET)
// requests when no custom set is given.
var defaultRetryStatuses = []int{408, 500, 502, 503, 504}

var headerNameRe = regexp.MustCompile(`^[A-Za-z0-9!#$%&'*+.^_` + "`" + `|~-]+$`)

// RetryContext is what a custom RetryOptions.ShouldRetry receives.
type RetryContext struct {
	// Err is the error of the failed attempt (a *Error or *RateLimitError).
	Err error
	// Attempt is the zero-based index of the attempt that just failed.
	Attempt int
	Method  string
	Path    string
	// DefaultDecision is what the built-in policy would decide.
	DefaultDecision bool
}

// RetryEvent is passed to RetryOptions.OnRetry right before the SDK waits.
type RetryEvent struct {
	Err         error
	RetryNumber int // one-based
	Delay       time.Duration
	Method      string
	Path        string
}

// ResponseEvent is passed to Options.OnResponse after every HTTP exchange.
type ResponseEvent struct {
	Method string
	Path   string
	Meta   ResponseMeta
}

// RetryOptions is fine-grained retry behaviour. Every field is optional.
type RetryOptions struct {
	// BaseDelay is the base of the exponential backoff. Defaults to 250ms.
	BaseDelay time.Duration
	// MaxDelay is the ceiling for a single backoff wait. Defaults to 2s.
	MaxDelay time.Duration
	// Statuses are retried for idempotent (GET) requests. Defaults to
	// {408, 500, 502, 503, 504}.
	Statuses []int
	// DisableRateLimitRetry turns off retrying a burst (per-minute) 429. By
	// default a burst 429 is retried honoring Retry-After; a monthly-quota 429
	// is never retried.
	DisableRateLimitRetry bool
	// ShouldRetry lets you override the decision. Return a non-nil pointer to
	// force true/false, or nil to keep the built-in policy. Retries stay bounded
	// by MaxRetries and MaxRetryDelay either way.
	ShouldRetry func(RetryContext) *bool
	// OnRetry is called before each retry wait. Handy for logging and metrics.
	OnRetry func(RetryEvent)
}

// Options configures New.
type Options struct {
	// APIKey is your Cotizave API key, sent as the X-API-Key header. If empty,
	// the COTIZAVE_API_KEY environment variable is used. Keep it server-side.
	APIKey string
	// BaseURL defaults to https://api.cotizave.com. Override only for testing.
	BaseURL string
	// Timeout is the per-request timeout. Defaults to 5s.
	Timeout time.Duration
	// HistoryTimeout is the timeout for history reads (History.* and
	// Rates.Snapshot), which can run longer. Defaults to 10s.
	HistoryTimeout time.Duration
	// MaxRetries is the maximum number of additional retries for eligible
	// requests. Zero means the default (2); set a negative value to disable
	// retries entirely.
	MaxRetries int
	// MaxRetryDelay is the total time budget across all retry waits for a single
	// call. Defaults to 10s.
	MaxRetryDelay time.Duration
	// Retry tunes backoff, retried statuses, and hooks.
	Retry RetryOptions
	// MaxResponseBytes caps the response body the client reads before failing
	// with a "response_too_large" error. Defaults to 25_000_000 (25 MB).
	MaxResponseBytes int64
	// Headers are extra headers sent with every request. They cannot set
	// X-API-Key. Per-call headers override these.
	Headers map[string]string
	// Language is sent as Accept-Language for localized fields ("en" or "es").
	Language string
	// UserAgent is an extra token appended to the SDK's User-Agent, identifying
	// your app (e.g. "acme-billing/1.2.3"). Never put secrets here.
	UserAgent string
	// AllowInsecureHTTP allows a non-HTTPS BaseURL. Off by default, because an
	// http base would send your API key in cleartext. Only for local testing.
	AllowInsecureHTTP bool
	// HTTPClient is a custom *http.Client (for tests, instrumentation, proxies
	// or custom TLS). The SDK always overrides its redirect policy to refuse
	// redirects and manages timeouts via context, leaving its Transport intact.
	HTTPClient *http.Client
	// OnResponse is called after every completed HTTP exchange, successful or
	// not. Use it to observe quota and rate-limit headers.
	OnResponse func(ResponseEvent)
}

// Config is the resolved, safe-to-log configuration exposed by Client.Config.
// It deliberately omits the API key, custom headers, hooks and the HTTP client.
type Config struct {
	BaseURL          string
	Timeout          time.Duration
	HistoryTimeout   time.Duration
	MaxRetries       int
	MaxRetryDelay    time.Duration
	MaxResponseBytes int64
	UserAgent        string
	Language         string
}

// resolvedRetry is the retry policy with defaults applied.
type resolvedRetry struct {
	baseDelay      time.Duration
	maxDelay       time.Duration
	statuses       []int
	rateLimitRetry bool
	shouldRetry    func(RetryContext) *bool
	onRetry        func(RetryEvent)
}

// config is the fully-resolved configuration, internal to the SDK.
type config struct {
	apiKey           string
	baseURL          string
	origin           string
	timeout          time.Duration
	historyTimeout   time.Duration
	maxRetries       int
	maxRetryDelay    time.Duration
	retry            resolvedRetry
	maxResponseBytes int64
	headers          map[string]string
	language         string
	userAgent        string
	onResponse       func(ResponseEvent)
}

// resolveConfig validates Options and applies defaults. It returns a plain
// error for misconfiguration — no request has been made yet.
func resolveConfig(opts Options) (*config, error) {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv(apiKeyEnvVar)
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("cotizave: an API key is required (set Options.APIKey or the %s environment variable)", apiKeyEnvVar)
	}

	baseURL, origin, err := resolveBaseURL(opts)
	if err != nil {
		return nil, err
	}

	headers, err := validateHeaders(opts.Headers, "Headers")
	if err != nil {
		return nil, err
	}

	language := opts.Language
	if language != "" {
		if err := validateHeaderValue(language, "Language"); err != nil {
			return nil, err
		}
	}

	userAgent := userAgentPrefix + " go/" + runtimeVersion()
	if opts.UserAgent != "" {
		if err := validateHeaderValue(opts.UserAgent, "UserAgent"); err != nil {
			return nil, err
		}
		userAgent = userAgent + " " + opts.UserAgent
	}

	cfg := &config{
		apiKey:           apiKey,
		baseURL:          baseURL,
		origin:           origin,
		timeout:          durationOr(opts.Timeout, DefaultTimeout),
		historyTimeout:   durationOr(opts.HistoryTimeout, DefaultHistoryTimeout),
		maxRetries:       resolveMaxRetries(opts.MaxRetries),
		maxRetryDelay:    durationOr(opts.MaxRetryDelay, DefaultMaxRetryDelay),
		retry:            resolveRetry(opts.Retry),
		maxResponseBytes: int64Or(opts.MaxResponseBytes, DefaultMaxResponseBytes),
		headers:          headers,
		language:         language,
		userAgent:        userAgent,
		onResponse:       opts.OnResponse,
	}
	return cfg, nil
}

func resolveMaxRetries(v int) int {
	if v == 0 {
		return DefaultMaxRetries
	}
	if v < 0 {
		return 0
	}
	return v
}

func durationOr(v, def time.Duration) time.Duration {
	if v <= 0 {
		return def
	}
	return v
}

func int64Or(v, def int64) int64 {
	if v <= 0 {
		return def
	}
	return v
}

func resolveRetry(r RetryOptions) resolvedRetry {
	base := durationOr(r.BaseDelay, DefaultRetryBaseDelay)
	max := durationOr(r.MaxDelay, DefaultRetryMaxDelay)
	if max < base {
		max = base
	}
	statuses := defaultRetryStatuses
	if r.Statuses != nil {
		statuses = append([]int(nil), r.Statuses...)
	}
	return resolvedRetry{
		baseDelay:      base,
		maxDelay:       max,
		statuses:       statuses,
		rateLimitRetry: !r.DisableRateLimitRetry,
		shouldRetry:    r.ShouldRetry,
		onRetry:        r.OnRetry,
	}
}

func resolveBaseURL(opts Options) (baseURL, origin string, err error) {
	raw := opts.BaseURL
	if raw == "" {
		raw = DefaultBaseURL
	}
	u, parseErr := url.Parse(raw)
	if parseErr != nil || u.Host == "" {
		return "", "", fmt.Errorf("cotizave: BaseURL must be a valid URL, got: %q", raw)
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !opts.AllowInsecureHTTP {
			return "", "", fmt.Errorf("cotizave: BaseURL uses http:// which would send your API key in cleartext; use https:// or set AllowInsecureHTTP for local testing")
		}
	default:
		return "", "", fmt.Errorf("cotizave: BaseURL must be an http(s) URL, got: %q", raw)
	}
	trimmed := strings.TrimRight(raw, "/")
	return trimmed, u.Scheme + "://" + u.Host, nil
}

// validateHeaders returns a clean copy of caller headers, rejecting bad names,
// control characters, and any attempt to set the reserved X-API-Key header.
func validateHeaders(headers map[string]string, optionName string) (map[string]string, error) {
	clean := make(map[string]string, len(headers))
	for name, value := range headers {
		if !headerNameRe.MatchString(name) {
			return nil, fmt.Errorf("cotizave: invalid header name in %s: %q", optionName, name)
		}
		if strings.EqualFold(name, reservedHdr) {
			return nil, fmt.Errorf("cotizave: %s cannot set the X-API-Key header; use Options.APIKey", optionName)
		}
		if err := validateHeaderValue(value, optionName+"."+name); err != nil {
			return nil, err
		}
		clean[name] = value
	}
	return clean, nil
}

// validateHeaderValue rejects CR/LF and other control characters (header
// injection). A conformant net/http rejects these too, but validate first.
func validateHeaderValue(value, optionName string) error {
	for i := 0; i < len(value); i++ {
		if c := value[i]; c < 0x20 || c == 0x7f {
			return fmt.Errorf("cotizave: %s must not contain control characters", optionName)
		}
	}
	return nil
}
