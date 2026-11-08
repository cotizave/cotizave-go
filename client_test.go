package cotizave

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// fmtSprintfPlus formats v with %+v — used to assert nothing leaks the key.
func fmtSprintfPlus(v any) string { return fmt.Sprintf("%+v", v) }

func TestVersionMatchesTagConvention(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(Version) {
		t.Errorf("Version %q is not a semver string (a release tag is vX.Y.Z)", Version)
	}
}

func TestErrorStringNeverIncludesKey(t *testing.T) {
	// Drive a real failing request and confirm the key is absent from the error.
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		fmt.Fprint(w, `{"code":"unauthorized","message":"bad key"}`)
	}, func(o *Options) { o.MaxRetries = -1 })
	_, err := c.Rates.Get(context.Background(), "CO", "co-trm")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), testKey) {
		t.Errorf("error string leaked the API key: %v", err)
	}
}

func TestClientIsUsableZeroValueOptions(t *testing.T) {
	// Only APIKey provided; everything else defaulted.
	c, err := New(Options{APIKey: testKey})
	if err != nil {
		t.Fatal(err)
	}
	if c.Rates == nil || c.History == nil || c.Crypto == nil {
		t.Error("resources not wired")
	}
}
