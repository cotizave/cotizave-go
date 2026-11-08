package cotizave

import (
	"testing"
	"time"
)

func policy() resolvedRetry { return resolveRetry(RetryOptions{}) }

func connErr() *Error        { return &Error{Status: 0, Code: "connection_error", isConnection: true} }
func sizeErr() *Error        { return &Error{Status: 0, Code: "response_too_large"} }
func statusErr(s int) *Error { return &Error{Status: s, Code: "x"} }

func burst429() error {
	e := &Error{Status: 429, Code: "rate_limited", Meta: &ResponseMeta{RateLimit: RateLimit{Scope: "burst"}}}
	return &RateLimitError{Err: e}
}
func monthly429() error {
	e := &Error{Status: 429, Code: "rate_limited", Details: map[string]any{"kind": "monthly"}}
	return &RateLimitError{Err: e}
}

func TestDefaultShouldRetry(t *testing.T) {
	p := policy()
	cases := []struct {
		name   string
		method string
		err    error
		want   bool
	}{
		{"GET connection", "GET", connErr(), true},
		{"GET size error", "GET", sizeErr(), false},
		{"GET 500", "GET", statusErr(500), true},
		{"GET 400", "GET", statusErr(400), false},
		{"GET 404", "GET", statusErr(404), false},
		{"GET burst 429", "GET", burst429(), true},
		{"GET monthly 429", "GET", monthly429(), false},
		{"POST 500", "POST", statusErr(500), false},
		{"POST connection", "POST", connErr(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := defaultShouldRetry(p, tc.method, tc.err); got != tc.want {
				t.Errorf("defaultShouldRetry = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestShouldRetryOverride(t *testing.T) {
	yes := true
	p := resolveRetry(RetryOptions{ShouldRetry: func(RetryContext) *bool { return &yes }})
	// Override forces a retry even for a 400.
	if !shouldRetry(p, "GET", "/x", statusErr(400), 0) {
		t.Error("custom ShouldRetry returning true should force a retry")
	}
	// Returning nil keeps the default (no retry on 400).
	p2 := resolveRetry(RetryOptions{ShouldRetry: func(RetryContext) *bool { return nil }})
	if shouldRetry(p2, "GET", "/x", statusErr(400), 0) {
		t.Error("nil override should keep the default decision")
	}
}

func TestNextDelayHonorsRetryAfterMs(t *testing.T) {
	p := policy()
	e := &Error{Status: 429, Meta: &ResponseMeta{RateLimit: RateLimit{Scope: "burst", RetryAfterMs: 5, hasRetryMs: true}}}
	err := &RateLimitError{Err: e}
	d := nextDelay(p, err, 0, time.Second)
	if d != 5*time.Millisecond {
		t.Errorf("delay = %v, want 5ms", d)
	}
}

func TestNextDelayRetryAfterBeyondBudgetStops(t *testing.T) {
	p := policy()
	e := &Error{Status: 429, Meta: &ResponseMeta{RateLimit: RateLimit{Scope: "burst", RetryAfter: 100, hasRetryAfter: true}}}
	err := &RateLimitError{Err: e}
	if d := nextDelay(p, err, 0, 50*time.Millisecond); d >= 0 {
		t.Errorf("delay = %v, want negative (stop) when Retry-After exceeds budget", d)
	}
}

func TestNextDelayBackoffWithinBudget(t *testing.T) {
	p := policy()
	for attempt := 0; attempt < 3; attempt++ {
		d := nextDelay(p, connErr(), attempt, time.Second)
		if d < 0 || d > p.maxDelay {
			t.Errorf("attempt %d: delay %v out of [0, %v]", attempt, d, p.maxDelay)
		}
	}
}

func TestNextDelayZeroBudgetStops(t *testing.T) {
	if d := nextDelay(policy(), connErr(), 0, 0); d >= 0 {
		t.Errorf("delay = %v, want negative when budget is 0", d)
	}
}
