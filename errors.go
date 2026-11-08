package cotizave

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// RateLimit holds the rate-limit metadata parsed from a response's headers.
type RateLimit struct {
	// Scope is which axis was hit on a 429: "monthly" (quota) or "burst".
	Scope string
	// Limit is the monthly quota ceiling (X-RateLimit-Limit).
	Limit int
	// Remaining is the monthly quota remaining (X-RateLimit-Remaining).
	Remaining int
	// Reset is when the monthly quota resets (X-RateLimit-Reset, RFC 3339).
	Reset string
	// LimitPerMinute is the per-minute velocity ceiling.
	LimitPerMinute int
	// RemainingPerMinute is the per-minute velocity remaining.
	RemainingPerMinute int
	// RetryAfter is Retry-After in whole seconds, if provided.
	RetryAfter int
	// RetryAfterMs is the sub-second retry hint for burst limits
	// (X-RateLimit-Retry-After-Ms).
	RetryAfterMs int

	hasLimit      bool
	hasRemaining  bool
	hasLimitMin   bool
	hasRemainMin  bool
	hasRetryAfter bool
	hasRetryMs    bool
}

// ResponseMeta is everything the SDK knows about a finished HTTP exchange.
type ResponseMeta struct {
	Status    int
	Headers   map[string]string
	RequestID string
	RateLimit RateLimit
}

// Error is the error type every API failure surfaces as. Catch it with
// errors.As to read the API-provided Code and Message, the HTTP Status, and the
// response metadata. A failure that never completed (network, timeout) has
// Status 0; Timeout reports whether it was a deadline.
//
// The API key is never included in an Error's message or fields.
type Error struct {
	// Status is the HTTP status code, or 0 when the request never completed.
	Status int
	// Code is the machine-readable error code from the API (e.g.
	// "rate_limit_exceeded"), or an SDK code like "timeout" / "connection_error".
	Code string
	// Message is a human-readable description. It never contains the API key.
	Message string
	// Field is the offending field, when the API points at one.
	Field string
	// Details is structured extra context from the API error body.
	Details map[string]any
	// Meta is the response metadata, including parsed rate-limit headers.
	Meta *ResponseMeta
	// Timeout reports whether the failure was a timeout (deadline exceeded).
	Timeout bool

	// isConnection marks a network/timeout failure (retryable), as opposed to a
	// deliberate abort or a response-size error.
	isConnection bool
	cause        error
}

// Error implements the error interface. It never exposes the API key.
func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("cotizave: ")
	if e.Message != "" {
		b.WriteString(e.Message)
	} else {
		b.WriteString("request failed")
	}
	b.WriteString(" (code=")
	b.WriteString(e.Code)
	if e.Status != 0 {
		b.WriteString(", status=")
		b.WriteString(strconv.Itoa(e.Status))
	}
	if e.Field != "" {
		b.WriteString(", field=")
		b.WriteString(e.Field)
	}
	b.WriteString(")")
	return b.String()
}

// Unwrap returns the underlying cause, if any.
func (e *Error) Unwrap() error { return e.cause }

// RateLimitInfo returns the parsed rate-limit metadata, or nil if the request
// never completed.
func (e *Error) RateLimitInfo() *RateLimit {
	if e.Meta == nil {
		return nil
	}
	return &e.Meta.RateLimit
}

// RateLimitError is returned for a 429 response. Use Scope to tell an exhausted
// monthly quota from a per-minute burst.
type RateLimitError struct {
	// Err is the underlying *Error (status 429). It is also reachable via
	// errors.As(err, &someError) because RateLimitError unwraps to it.
	Err *Error
}

// Error implements the error interface.
func (e *RateLimitError) Error() string { return e.Err.Error() }

// Unwrap exposes the underlying *Error so errors.As and the status predicates
// work on a rate-limit error too.
func (e *RateLimitError) Unwrap() error { return e.Err }

// Scope reports "monthly" (quota exhausted — do not retry soon) or "burst"
// (slow down), or "" when unknown.
func (e *RateLimitError) Scope() string {
	if e.Err.Details != nil {
		if kind, ok := e.Err.Details["kind"].(string); ok && (kind == "monthly" || kind == "burst") {
			return kind
		}
	}
	if rl := e.Err.RateLimitInfo(); rl != nil {
		return rl.Scope
	}
	return ""
}

// RetryAfter returns Retry-After in seconds, if provided.
func (e *RateLimitError) RetryAfter() int {
	if rl := e.Err.RateLimitInfo(); rl != nil {
		return rl.RetryAfter
	}
	return 0
}

// IsQuotaExhausted reports whether the monthly quota is exhausted (waiting a
// few seconds will not help).
func (e *RateLimitError) IsQuotaExhausted() bool { return e.Scope() == "monthly" }

// apiErrorBody is the shape of the API's JSON error payload.
type apiErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Field   string         `json:"field"`
	Details map[string]any `json:"details"`
}

var dangerousDetailKeys = map[string]struct{}{
	"__proto__":   {},
	"constructor": {},
	"prototype":   {},
}

// sanitizeDetails strips prototype-polluting keys from server-provided details,
// defensively, so a consumer that later merges them elsewhere is not exposed.
func sanitizeDetails(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if _, bad := dangerousDetailKeys[k]; bad {
			continue
		}
		out[k] = v
	}
	return out
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// errorFromResponse builds the right error from a completed HTTP response.
// body is the raw (already size-capped) response body.
func errorFromResponse(status int, body []byte, meta ResponseMeta) error {
	var parsed apiErrorBody
	hasAPIError := false
	if len(body) > 0 {
		if err := json.Unmarshal(body, &parsed); err == nil && parsed.Code != "" {
			hasAPIError = true
		}
	}

	code := parsed.Code
	message := parsed.Message
	var details map[string]any

	if hasAPIError {
		details = sanitizeDetails(parsed.Details)
	} else {
		code = fallbackCode(status)
		message = fallbackMessage(status)
		if len(body) > 0 {
			details = map[string]any{"non_json_body": truncate(string(body), 500)}
		}
	}

	metaCopy := meta
	e := &Error{
		Status:  status,
		Code:    code,
		Message: message,
		Meta:    &metaCopy,
	}
	if hasAPIError {
		e.Field = parsed.Field
	}
	if details != nil {
		e.Details = details
	}

	if status == 429 {
		return &RateLimitError{Err: e}
	}
	return e
}

func fallbackCode(status int) string {
	switch {
	case status >= 500:
		return "server_error"
	case status == 404:
		return "resource_not_found"
	default:
		return "http_error"
	}
}

func fallbackMessage(status int) string {
	return fmt.Sprintf("Unexpected HTTP %d response from Cotizave.", status)
}

// --- Ergonomic predicates (work through wrapping via errors.As) ---

// AsError extracts the underlying *Error from err, if present.
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// AsRateLimitError extracts a *RateLimitError from err, if present.
func AsRateLimitError(err error) (*RateLimitError, bool) {
	var e *RateLimitError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// StatusCode returns the HTTP status of a Cotizave error, or 0 if err is not
// one (or never completed).
func StatusCode(err error) int {
	if e, ok := AsError(err); ok {
		return e.Status
	}
	return 0
}

// ErrorCode returns the machine-readable code of a Cotizave error, or "".
func ErrorCode(err error) string {
	if e, ok := AsError(err); ok {
		return e.Code
	}
	return ""
}

// IsBadRequest reports a 400.
func IsBadRequest(err error) bool { return StatusCode(err) == 400 }

// IsAuth reports a 401 (missing, invalid or revoked API key).
func IsAuth(err error) bool { return StatusCode(err) == 401 }

// IsFeatureNotInPlan reports a 402 (feature not included in the plan).
func IsFeatureNotInPlan(err error) bool { return StatusCode(err) == 402 }

// IsForbidden reports a 403.
func IsForbidden(err error) bool { return StatusCode(err) == 403 }

// IsNotFound reports a 404.
func IsNotFound(err error) bool { return StatusCode(err) == 404 }

// IsNotAcceptable reports a 406.
func IsNotAcceptable(err error) bool { return StatusCode(err) == 406 }

// IsUnprocessable reports a 422.
func IsUnprocessable(err error) bool { return StatusCode(err) == 422 }

// IsRateLimited reports a 429.
func IsRateLimited(err error) bool {
	_, ok := AsRateLimitError(err)
	return ok
}

// IsServerError reports a 5xx.
func IsServerError(err error) bool { return StatusCode(err) >= 500 }

// IsTimeout reports whether the request failed because it exceeded its timeout.
func IsTimeout(err error) bool {
	if e, ok := AsError(err); ok {
		return e.Timeout
	}
	return false
}

// IsConnectionError reports a failure that never received a response (DNS, TCP,
// TLS, timeout, refused redirect).
func IsConnectionError(err error) bool {
	if e, ok := AsError(err); ok {
		return e.Status == 0 && e.isConnection
	}
	return false
}
