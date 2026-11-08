package cotizave

import (
	"strings"
	"testing"
	"time"
)

func TestNewRequiresKey(t *testing.T) {
	t.Setenv(apiKeyEnvVar, "")
	_, err := New(Options{})
	if err == nil {
		t.Fatal("expected an error when no API key is set")
	}
	if !strings.Contains(err.Error(), "API key is required") {
		t.Errorf("err = %v", err)
	}
}

func TestNewReadsEnvVar(t *testing.T) {
	t.Setenv(apiKeyEnvVar, "ctz_from_env")
	c, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.t.cfg.apiKey != "ctz_from_env" {
		t.Errorf("apiKey = %q", c.t.cfg.apiKey)
	}
}

func TestDefaultsApplied(t *testing.T) {
	c, err := New(Options{APIKey: testKey})
	if err != nil {
		t.Fatal(err)
	}
	cfg := c.Config()
	if cfg.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v", cfg.Timeout)
	}
	if cfg.HistoryTimeout != DefaultHistoryTimeout {
		t.Errorf("HistoryTimeout = %v", cfg.HistoryTimeout)
	}
	if cfg.MaxRetries != DefaultMaxRetries {
		t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, DefaultMaxRetries)
	}
	if cfg.MaxResponseBytes != DefaultMaxResponseBytes {
		t.Errorf("MaxResponseBytes = %d", cfg.MaxResponseBytes)
	}
}

func TestMaxRetriesDisableWithNegative(t *testing.T) {
	c, err := New(Options{APIKey: testKey, MaxRetries: -1})
	if err != nil {
		t.Fatal(err)
	}
	if c.Config().MaxRetries != 0 {
		t.Errorf("MaxRetries = %d, want 0 (disabled)", c.Config().MaxRetries)
	}
}

func TestHTTPBaseURLRejectedUnlessOptIn(t *testing.T) {
	_, err := New(Options{APIKey: testKey, BaseURL: "http://insecure.example.com"})
	if err == nil {
		t.Fatal("expected http:// to be rejected")
	}
	if !strings.Contains(err.Error(), "cleartext") {
		t.Errorf("err = %v", err)
	}
	// With the opt-in it is allowed.
	if _, err := New(Options{APIKey: testKey, BaseURL: "http://insecure.example.com", AllowInsecureHTTP: true}); err != nil {
		t.Errorf("AllowInsecureHTTP should permit http: %v", err)
	}
}

func TestInvalidBaseURLScheme(t *testing.T) {
	_, err := New(Options{APIKey: testKey, BaseURL: "ftp://example.com"})
	if err == nil {
		t.Fatal("expected a scheme error")
	}
}

func TestHeadersCannotSetAPIKey(t *testing.T) {
	_, err := New(Options{APIKey: testKey, Headers: map[string]string{"X-API-Key": "nope"}})
	if err == nil {
		t.Fatal("expected an error for setting X-API-Key via Headers")
	}
	if !strings.Contains(err.Error(), "X-API-Key") {
		t.Errorf("err = %v", err)
	}
}

func TestConfigAndStringNeverExposeKey(t *testing.T) {
	c, err := New(Options{APIKey: "ctz_super_secret_123", BaseURL: "https://api.cotizave.com"})
	if err != nil {
		t.Fatal(err)
	}
	// Config() is a struct with no key field; format it fully.
	if s := strings.ToLower(fmtSprintfPlus(c.Config())); strings.Contains(s, "secret") {
		t.Errorf("Config() leaked the key: %s", s)
	}
	for _, s := range []string{c.String(), c.GoString(), fmtSprintfPlus(c)} {
		if strings.Contains(s, "ctz_super_secret_123") {
			t.Errorf("client representation leaked the key: %s", s)
		}
	}
}

func TestUserAgentAppend(t *testing.T) {
	c, err := New(Options{APIKey: testKey, UserAgent: "acme/1.0"})
	if err != nil {
		t.Fatal(err)
	}
	ua := c.Config().UserAgent
	if !strings.HasPrefix(ua, "cotizave-sdk-go/"+Version) || !strings.HasSuffix(ua, "acme/1.0") {
		t.Errorf("UserAgent = %q", ua)
	}
}

func TestResolveRetryClampsMaxBelowBase(t *testing.T) {
	r := resolveRetry(RetryOptions{BaseDelay: 2 * time.Second, MaxDelay: 1 * time.Second})
	if r.maxDelay < r.baseDelay {
		t.Errorf("maxDelay %v < baseDelay %v", r.maxDelay, r.baseDelay)
	}
}
