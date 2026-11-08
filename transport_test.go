package cotizave

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testKey = "ctz_test_SECRET_key_value"

// newTestClient spins an httptest server and a client pointed at it.
func newTestClient(t *testing.T, handler http.HandlerFunc, mutate ...func(*Options)) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	opts := Options{
		APIKey:            testKey,
		BaseURL:           srv.URL,
		AllowInsecureHTTP: true,
	}
	for _, m := range mutate {
		m(&opts)
	}
	c, err := New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, srv
}

func TestGetSendsKeyAndParsesJSON(t *testing.T) {
	var gotKey, gotPath, gotAccept, gotUA string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		gotPath = r.URL.Path
		gotAccept = r.Header.Get("Accept")
		gotUA = r.Header.Get("User-Agent")
		fmt.Fprint(w, `{"id":"co-trm","mid":"4100.5","type":"reference"}`)
	})
	rate, err := c.Rates.Get(context.Background(), "CO", "co-trm")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if gotKey != testKey {
		t.Errorf("X-API-Key = %q, want %q", gotKey, testKey)
	}
	if gotPath != "/v2/fx/co/rates/co-trm" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q", gotAccept)
	}
	if !strings.HasPrefix(gotUA, "cotizave-sdk-go/") {
		t.Errorf("User-Agent = %q", gotUA)
	}
	if rate.ID != "co-trm" || rate.Mid.String() != "4100.5" {
		t.Errorf("rate = %+v", rate)
	}
}

func TestRefusesRedirectAndDoesNotForwardKey(t *testing.T) {
	var targetHit int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&targetHit, 1)
		fmt.Fprint(w, `{"leaked":true}`)
	}))
	t.Cleanup(target.Close)

	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/v2/fx/co/rates/co-trm", http.StatusFound)
	})

	_, err := c.Rates.Get(context.Background(), "CO", "co-trm")
	if err == nil {
		t.Fatal("expected an error on redirect, got nil")
	}
	if code := ErrorCode(err); code != "unexpected_redirect" {
		t.Errorf("code = %q, want unexpected_redirect", code)
	}
	if n := atomic.LoadInt32(&targetHit); n != 0 {
		t.Errorf("redirect target was hit %d times; the key must never be forwarded", n)
	}
	if !IsConnectionError(err) {
		// unexpected_redirect is a non-retryable connection-class failure
		if _, ok := AsError(err); !ok {
			t.Errorf("err is not a *Error: %v", err)
		}
	}
}

func TestResponseSizeCap(t *testing.T) {
	big := strings.Repeat("a", 5000)
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"blob":%q}`, big)
	}, func(o *Options) { o.MaxResponseBytes = 1000 })

	_, err := c.Rates.Get(context.Background(), "CO", "co-trm")
	if err == nil {
		t.Fatal("expected response_too_large, got nil")
	}
	if code := ErrorCode(err); code != "response_too_large" {
		t.Errorf("code = %q, want response_too_large", code)
	}
}

func TestOriginPinningRejectsOffOriginPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server must not be reached for an off-origin path, path=%s", r.URL.Path)
	})
	_, err := c.Request(context.Background(), RequestOptions{Path: "@evil.example.com/v2/fx/co/rates"})
	if err == nil {
		t.Fatal("expected invalid_request_path, got nil")
	}
	if code := ErrorCode(err); code != "invalid_request_path" {
		t.Errorf("code = %q, want invalid_request_path", code)
	}
}

func TestPerCallHeaderCannotOverrideAPIKey(t *testing.T) {
	var gotKey string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		fmt.Fprint(w, `{}`)
	})
	_, err := c.Rates.Get(context.Background(), "CO", "co-trm", RequestConfig{
		Headers: map[string]string{"X-API-Key": "attacker"},
	})
	if err == nil {
		t.Fatal("expected invalid_header_value error, got nil")
	}
	if code := ErrorCode(err); code != "invalid_header_value" {
		t.Errorf("code = %q, want invalid_header_value", code)
	}
	if gotKey == "attacker" {
		t.Error("X-API-Key was overridden by a per-call header")
	}
}

func TestHeaderInjectionRejected(t *testing.T) {
	_, err := New(Options{
		APIKey:  testKey,
		BaseURL: "https://api.cotizave.com",
		Headers: map[string]string{"X-Trace": "line1\r\nX-Injected: evil"},
	})
	if err == nil {
		t.Fatal("expected a control-character error, got nil")
	}
	if !strings.Contains(err.Error(), "control characters") {
		t.Errorf("err = %v", err)
	}
}

func TestErrorMappingPerStatus(t *testing.T) {
	cases := []struct {
		status int
		check  func(error) bool
		name   string
	}{
		{400, IsBadRequest, "400"},
		{401, IsAuth, "401"},
		{402, IsFeatureNotInPlan, "402"},
		{403, IsForbidden, "403"},
		{404, IsNotFound, "404"},
		{406, IsNotAcceptable, "406"},
		{422, IsUnprocessable, "422"},
		{500, IsServerError, "500"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprintf(w, `{"code":"err_%d","message":"boom","field":"amount"}`, tc.status)
			}, func(o *Options) { o.MaxRetries = -1 })
			_, err := c.Rates.Get(context.Background(), "CO", "co-trm")
			if err == nil {
				t.Fatalf("status %d: expected error", tc.status)
			}
			if !tc.check(err) {
				t.Errorf("status %d: predicate failed, err=%v", tc.status, err)
			}
			e, ok := AsError(err)
			if !ok {
				t.Fatalf("not a *Error: %v", err)
			}
			if e.Status != tc.status || e.Code != fmt.Sprintf("err_%d", tc.status) || e.Field != "amount" {
				t.Errorf("mapped error = %+v", e)
			}
		})
	}
}

func TestNonJSONErrorBodyFallback(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(502)
		fmt.Fprint(w, "<html>bad gateway</html>")
	}, func(o *Options) { o.MaxRetries = -1 })
	_, err := c.Rates.Get(context.Background(), "CO", "co-trm")
	e, ok := AsError(err)
	if !ok {
		t.Fatalf("not a *Error: %v", err)
	}
	if e.Code != "server_error" {
		t.Errorf("code = %q, want server_error", e.Code)
	}
	if e.Details["non_json_body"] == nil {
		t.Errorf("expected non_json_body in details, got %+v", e.Details)
	}
}

func TestBurst429RetriedMonthly429Not(t *testing.T) {
	// Burst 429 then 200: should retry and succeed.
	var calls int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("X-RateLimit-Scope", "burst")
			w.Header().Set("X-RateLimit-Retry-After-Ms", "5")
			w.WriteHeader(429)
			fmt.Fprint(w, `{"code":"rate_limited","message":"slow down"}`)
			return
		}
		fmt.Fprint(w, `{"id":"co-trm"}`)
	})
	if _, err := c.Rates.Get(context.Background(), "CO", "co-trm"); err != nil {
		t.Fatalf("burst 429 should be retried: %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Errorf("calls = %d, want 2 (one retry)", n)
	}

	// Monthly 429: must NOT be retried.
	var mcalls int32
	c2, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&mcalls, 1)
		w.Header().Set("X-RateLimit-Scope", "monthly")
		w.WriteHeader(429)
		fmt.Fprint(w, `{"code":"rate_limited","message":"quota","details":{"kind":"monthly"}}`)
	})
	_, err := c2.Rates.Get(context.Background(), "CO", "co-trm")
	if err == nil {
		t.Fatal("monthly 429 should error")
	}
	rle, ok := AsRateLimitError(err)
	if !ok {
		t.Fatalf("not a *RateLimitError: %v", err)
	}
	if !rle.IsQuotaExhausted() {
		t.Errorf("Scope=%q, want monthly", rle.Scope())
	}
	if n := atomic.LoadInt32(&mcalls); n != 1 {
		t.Errorf("monthly calls = %d, want 1 (no retry)", n)
	}
}

func TestPOSTNotRetried(t *testing.T) {
	var calls int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(503)
		fmt.Fprint(w, `{"code":"server_error","message":"down"}`)
	})
	_, err := c.ConvertBulk(context.Background(), BulkConvertOptions{
		Conversions: []BulkConvertItemInput{{From: "USD", To: "ARS", Amount: 1}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("POST calls = %d, want 1 (POST is not retried)", n)
	}
}

func TestRetryBudgetRespected(t *testing.T) {
	var calls int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("X-RateLimit-Scope", "burst")
		w.Header().Set("Retry-After", "100") // 100s, far beyond the budget
		w.WriteHeader(429)
		fmt.Fprint(w, `{"code":"rate_limited","message":"slow"}`)
	}, func(o *Options) { o.MaxRetryDelay = 50 * time.Millisecond })
	_, err := c.Rates.Get(context.Background(), "CO", "co-trm")
	if err == nil {
		t.Fatal("expected error")
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("calls = %d, want 1 (budget forbids the long wait)", n)
	}
}

func TestContextCancellationStopsRetry(t *testing.T) {
	var calls int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("X-RateLimit-Scope", "burst")
		w.Header().Set("X-RateLimit-Retry-After-Ms", "1000")
		w.WriteHeader(429)
		fmt.Fprint(w, `{"code":"rate_limited","message":"slow"}`)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := c.Rates.Get(ctx, "CO", "co-trm")
	if err == nil {
		t.Fatal("expected error from canceled context")
	}
}

func TestTimeout(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(60 * time.Millisecond)
		fmt.Fprint(w, `{}`)
	}, func(o *Options) {
		o.Timeout = 15 * time.Millisecond
		o.MaxRetries = -1
	})
	_, err := c.Rates.Get(context.Background(), "CO", "co-trm")
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if !IsTimeout(err) {
		t.Errorf("IsTimeout=false, err=%v", err)
	}
}

func Test304NotModified(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"abc"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		fmt.Fprint(w, `{"id":"co-trm"}`)
	})
	res, err := c.Request(context.Background(), RequestOptions{
		Path:        "/v2/fx/co/rates/co-trm",
		IfNoneMatch: `"abc"`,
	})
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if !res.NotModified {
		t.Errorf("NotModified=false, want true")
	}
}

func TestExportCSV(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if a := r.Header.Get("Accept"); a != "text/csv" {
			t.Errorf("Accept = %q, want text/csv", a)
		}
		if f := r.URL.Query().Get("format"); f != "csv" {
			t.Errorf("format = %q", f)
		}
		fmt.Fprint(w, "id,mid\nco-trm,4100.5\n")
	})
	body, err := c.Export(context.Background(), "/v2/fx/co/rates", "csv", nil)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if !strings.Contains(body, "co-trm,4100.5") {
		t.Errorf("body = %q", body)
	}
}

func TestHistoryUsesLongerTimeout(t *testing.T) {
	// A ~40ms server response fails under the 15ms default timeout but passes
	// under the 200ms history timeout — proving history reads use the latter.
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(40 * time.Millisecond)
		fmt.Fprint(w, `{"id":"co-trm","points":[]}`)
	}, func(o *Options) {
		o.Timeout = 15 * time.Millisecond
		o.HistoryTimeout = 200 * time.Millisecond
		o.MaxRetries = -1
	})
	if _, err := c.History.Daily(context.Background(), HistoryDailyOptions{Country: "CO", RateID: "co-trm"}); err != nil {
		t.Fatalf("history read should use the longer timeout: %v", err)
	}
	// A live rate read with the same server should time out.
	if _, err := c.Rates.Get(context.Background(), "CO", "co-trm"); !IsTimeout(err) {
		t.Errorf("live read should time out under the short timeout, err=%v", err)
	}
}

func TestOnResponseHook(t *testing.T) {
	var seen int32
	var gotRemaining int
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "123")
		w.Header().Set("X-Request-Id", "req-7")
		fmt.Fprint(w, `{}`)
	}, func(o *Options) {
		o.OnResponse = func(e ResponseEvent) {
			atomic.AddInt32(&seen, 1)
			gotRemaining = e.Meta.RateLimit.Remaining
			if e.Meta.RequestID != "req-7" {
				t.Errorf("RequestID = %q", e.Meta.RequestID)
			}
		}
	})
	if _, err := c.Rates.Get(context.Background(), "CO", "co-trm"); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&seen) != 1 || gotRemaining != 123 {
		t.Errorf("hook seen=%d remaining=%d", seen, gotRemaining)
	}
}
