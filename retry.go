package cotizave

import (
	"errors"
	"math/rand/v2"
	"time"
)

// defaultShouldRetry is the built-in decision, before any custom ShouldRetry.
//
// Only idempotent (GET) requests are retried. A 429 is retried only when it is
// a burst (velocity) limit — never when the monthly quota is exhausted. A
// failure that never received a response (network/timeout) is retried. Errors
// that require changing the request (400/401/402/403/404/406/422) are not.
func defaultShouldRetry(p resolvedRetry, method string, err error) bool {
	if method != "GET" {
		return false
	}
	if rle, ok := AsRateLimitError(err); ok {
		return p.rateLimitRetry && rle.Scope() == "burst"
	}
	e, ok := AsError(err)
	if !ok {
		return false
	}
	if e.Status == 0 {
		return e.isConnection // network/timeout, but not a size error or abort
	}
	for _, s := range p.statuses {
		if s == e.Status {
			return true
		}
	}
	return false
}

// shouldRetry is the final decision: a custom ShouldRetry wins when it returns
// a non-nil pointer.
func shouldRetry(p resolvedRetry, method, path string, err error, attempt int) bool {
	base := defaultShouldRetry(p, method, err)
	if p.shouldRetry == nil {
		return base
	}
	if override := p.shouldRetry(RetryContext{
		Err:             err,
		Attempt:         attempt,
		Method:          method,
		Path:            path,
		DefaultDecision: base,
	}); override != nil {
		return *override
	}
	return base
}

// nextDelay computes how long to wait before the next attempt. It honors a
// server Retry-After (seconds) or the sub-second X-RateLimit-Retry-After-Ms
// when present; otherwise exponential backoff with full jitter. It returns a
// negative duration when the wait would exceed the remaining budget (stop).
func nextDelay(p resolvedRetry, err error, attempt int, budgetLeft time.Duration) time.Duration {
	if budgetLeft <= 0 {
		return -1
	}

	var rle *RateLimitError
	if errors.As(err, &rle) {
		if rl := rle.Err.RateLimitInfo(); rl != nil {
			hintMs := -1
			switch {
			case rl.hasRetryMs:
				hintMs = rl.RetryAfterMs
			case rl.hasRetryAfter:
				hintMs = rl.RetryAfter * 1000
			}
			if hintMs >= 0 {
				d := time.Duration(hintMs) * time.Millisecond
				if d <= budgetLeft {
					return d
				}
				return -1
			}
		}
	}

	// Exponential backoff with full jitter, capped per wait and by the budget.
	ceiling := p.maxDelay
	if attempt < 30 {
		scaled := p.baseDelay << uint(attempt)
		if scaled < p.maxDelay {
			ceiling = scaled
		}
	}
	if ceiling <= 0 {
		return 0
	}
	jitter := time.Duration(rand.Int64N(int64(ceiling) + 1))
	if jitter > budgetLeft {
		return budgetLeft
	}
	return jitter
}
