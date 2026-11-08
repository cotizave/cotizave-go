package cotizave

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestHistoryIterate(t *testing.T) {
	page := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			fmt.Fprint(w, `{"points":[{"date":"2026-01-01","mid":"1"},{"date":"2026-01-02","mid":"2"}],"next_cursor":"c2"}`)
			return
		}
		fmt.Fprint(w, `{"points":[{"date":"2026-01-03","mid":"3"}]}`)
	})
	var dates []string
	err := c.History.Iterate(context.Background(), HistoryIterateOptions{Country: "VE", RateID: "ve-bcv"}, func(p HistoryPointRow) bool {
		dates = append(dates, p.Date)
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(dates) != 3 || dates[2] != "2026-01-03" {
		t.Errorf("dates = %v", dates)
	}
}

func TestHistoryIterateEarlyStop(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"points":[{"date":"a"},{"date":"b"}],"next_cursor":"more"}`)
	})
	count := 0
	err := c.History.Iterate(context.Background(), HistoryIterateOptions{Country: "VE", RateID: "ve-bcv", MaxPages: 3}, func(p HistoryPointRow) bool {
		count++
		return false // stop after the first point
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1 (early stop)", count)
	}
}

func TestCurrenciesAndHolidaysNextAndCountryGet(t *testing.T) {
	c, rec := newRecorderClient(t, `{"currencies":[{"code":"USD"}]}`)
	ctx := context.Background()
	if _, err := c.Currencies.List(ctx); err != nil {
		t.Fatal(err)
	}
	if rec.path != "/v2/currencies" {
		t.Errorf("path = %q", rec.path)
	}
	if _, err := c.Holidays.Next(ctx, "AR", "2026-05-01"); err != nil {
		t.Fatal(err)
	}
	if rec.path != "/v2/holidays/ar" || rec.query != "from=2026-05-01" {
		t.Errorf("next path=%q query=%q", rec.path, rec.query)
	}
}

func TestRawResultDecode(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":"co-trm","mid":"4100"}`)
	})
	res, err := c.Request(context.Background(), RequestOptions{Path: "/v2/fx/co/rates/co-trm"})
	if err != nil {
		t.Fatal(err)
	}
	var rate Rate
	if err := res.Decode(&rate); err != nil {
		t.Fatal(err)
	}
	if rate.ID != "co-trm" {
		t.Errorf("rate = %+v", rate)
	}
	// Empty body decode is a no-op.
	empty := &RawResult{}
	if err := empty.Decode(&rate); err != nil {
		t.Errorf("empty Decode = %v", err)
	}
}

func TestErrorPredicatesOnNonCotizaveError(t *testing.T) {
	err := errors.New("some other error")
	if StatusCode(err) != 0 || ErrorCode(err) != "" {
		t.Error("non-cotizave error should report 0 / \"\"")
	}
	if IsNotFound(err) || IsRateLimited(err) || IsTimeout(err) || IsConnectionError(err) {
		t.Error("predicates should be false for a non-cotizave error")
	}
	if _, ok := AsError(err); ok {
		t.Error("AsError should be false")
	}
}

func TestRateLimitErrorMethods(t *testing.T) {
	e := &Error{
		Status:  429,
		Code:    "rate_limited",
		Message: "slow down",
		Meta:    &ResponseMeta{RateLimit: RateLimit{Scope: "burst", RetryAfter: 7, hasRetryAfter: true}},
	}
	rle := &RateLimitError{Err: e}
	if rle.Error() == "" || rle.Scope() != "burst" || rle.RetryAfter() != 7 || rle.IsQuotaExhausted() {
		t.Errorf("rle = %+v scope=%s retry=%d", rle, rle.Scope(), rle.RetryAfter())
	}
	// errors.As must reach the embedded *Error for status predicates.
	var inner *Error
	if !errors.As(error(rle), &inner) || inner.Status != 429 {
		t.Error("errors.As should unwrap RateLimitError to *Error")
	}
	if !IsRateLimited(error(rle)) {
		t.Error("IsRateLimited should be true")
	}
}

func TestExportInvalidFormat(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be reached for an invalid format")
	})
	if _, err := c.Export(context.Background(), "/v2/fx/co/rates", "pdf", nil); err == nil {
		t.Error("invalid export format should error")
	}
}

func TestResourceCountryValidationPropagates(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be reached when the country code is empty")
	})
	ctx := context.Background()
	if _, err := c.Markets(ctx, ""); err == nil {
		t.Error("empty country to Markets should error")
	}
	if _, err := c.History.Daily(ctx, HistoryDailyOptions{Country: "", RateID: "x"}); err == nil {
		t.Error("empty country to history should error")
	}
	if _, err := c.Crypto.Get(ctx, ""); err == nil {
		t.Error("empty crypto id should error")
	}
}
