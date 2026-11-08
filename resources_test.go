package cotizave

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

// recorder captures the last request line and replies with a fixed JSON body.
type recorder struct {
	method string
	path   string
	query  string
}

func newRecorderClient(t *testing.T, reply string) (*Client, *recorder) {
	t.Helper()
	rec := &recorder{}
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.query = r.URL.RawQuery
		if reply == "" {
			reply = "{}"
		}
		fmt.Fprint(w, reply)
	})
	return c, rec
}

func TestResourcePathsAndQueries(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		call      func(*Client) error
		wantPath  string
		wantQuery string
		wantMeth  string
	}{
		{"rates.list", func(c *Client) error {
			_, err := c.Rates.List(ctx, "AR", &ListRatesOptions{Tag: "bcv", Base: "USD"})
			return err
		}, "/v2/fx/ar/rates", "base=USD&tag=bcv", "GET"},
		{"rates.get", func(c *Client) error {
			_, err := c.Rates.Get(ctx, "CO", "co-trm")
			return err
		}, "/v2/fx/co/rates/co-trm", "", "GET"},
		{"rates.snapshot", func(c *Client) error {
			_, err := c.Rates.Snapshot(ctx, "AR", "2026-10-01")
			return err
		}, "/v2/fx/ar/rates", "date=2026-10-01", "GET"},
		{"rates.global", func(c *Client) error {
			_, err := c.Rates.Global(ctx, &GlobalRatesOptions{Symbols: []string{"BRL", "EUR"}})
			return err
		}, "/v2/fx/rates", "symbols=BRL%2CEUR", "GET"},
		{"history.get", func(c *Client) error {
			_, err := c.History.Get(ctx, HistoryPointOptions{Country: "CO", RateID: "co-trm", Date: "2026-10-01"})
			return err
		}, "/v2/fx/co/rates/co-trm/history", "date=2026-10-01", "GET"},
		{"history.daily", func(c *Client) error {
			_, err := c.History.Daily(ctx, HistoryDailyOptions{Country: "AR", RateID: "ar-official", From: "2026-09-01", To: "2026-09-30"})
			return err
		}, "/v2/fx/ar/rates/ar-official/history", "from=2026-09-01&granularity=daily&to=2026-09-30", "GET"},
		{"history.stats", func(c *Client) error {
			_, err := c.History.Stats(ctx, HistoryStatsOptions{Country: "AR", RateID: "ar-official", MovingAverage: 7})
			return err
		}, "/v2/fx/ar/rates/ar-official/history", "ma=7&stats=1", "GET"},
		{"history.raw", func(c *Client) error {
			_, err := c.History.Raw(ctx, HistoryRawOptions{Country: "VE", RateID: "ve-bcv", Limit: 100})
			return err
		}, "/v2/fx/ve/rates/ve-bcv/history", "granularity=raw&limit=100", "GET"},
		{"history.limits", func(c *Client) error {
			_, err := c.History.Limits(ctx)
			return err
		}, "/v2/fx/historical-limits", "", "GET"},
		{"convert", func(c *Client) error {
			_, err := c.Convert(ctx, ConvertOptions{From: "USD", To: "BRL", Amount: 100})
			return err
		}, "/v2/fx/convert", "amount=100&from=USD&to=BRL", "GET"},
		{"markets", func(c *Client) error {
			_, err := c.Markets(ctx, "VE")
			return err
		}, "/v2/fx/ve/markets", "", "GET"},
		{"spread", func(c *Client) error {
			_, err := c.Spread(ctx, "VE", &SpreadOptions{Against: "ve-paralelo"})
			return err
		}, "/v2/fx/ve/spread", "against=ve-paralelo", "GET"},
		{"route", func(c *Client) error {
			_, err := c.Route(ctx, "VE", RouteOptions{From: "USD", To: "VES", Amount: 50, Direction: "send"})
			return err
		}, "/v2/fx/ve/route", "amount=50&direction=send&from=USD&to=VES", "GET"},
		{"bankRates", func(c *Client) error {
			_, err := c.BankRates(ctx, "VE", &BankRatesOptions{Bank: "0102"})
			return err
		}, "/v2/fx/ve/bank-rates", "bank=0102", "GET"},
		{"countries.list", func(c *Client) error {
			_, err := c.Countries.List(ctx)
			return err
		}, "/v2/countries", "", "GET"},
		{"countries.get", func(c *Client) error {
			_, err := c.Countries.Get(ctx, "AR")
			return err
		}, "/v2/countries/ar", "", "GET"},
		{"currencies.get", func(c *Client) error {
			_, err := c.Currencies.Get(ctx, "USD")
			return err
		}, "/v2/currencies/USD", "", "GET"},
		{"banks.list", func(c *Client) error {
			_, err := c.Banks.List(ctx, "VE", &BanksListOptions{Q: "mercantil"})
			return err
		}, "/v2/banks/ve", "q=mercantil", "GET"},
		{"banks.get", func(c *Client) error {
			_, err := c.Banks.Get(ctx, "VE", "0105")
			return err
		}, "/v2/banks/ve/0105", "", "GET"},
		{"issuers.list", func(c *Client) error {
			_, err := c.Issuers.List(ctx)
			return err
		}, "/v2/issuers", "", "GET"},
		{"holidays.list", func(c *Client) error {
			_, err := c.Holidays.List(ctx, "AR", &HolidaysListOptions{Year: 2026})
			return err
		}, "/v2/holidays/ar", "year=2026", "GET"},
		{"holidays.check", func(c *Client) error {
			_, err := c.Holidays.Check(ctx, "AR", "2026-05-01")
			return err
		}, "/v2/holidays/ar", "date=2026-05-01", "GET"},
		{"holidays.businessDays", func(c *Client) error {
			_, err := c.Holidays.BusinessDays(ctx, "VE", BusinessDaysOptions{From: "2026-10-01", Days: 5})
			return err
		}, "/v2/holidays/ve/business-days", "days=5&from=2026-10-01", "GET"},
		{"crypto.list", func(c *Client) error {
			_, err := c.Crypto.List(ctx, &CryptoListOptions{Tag: "usdt"})
			return err
		}, "/v2/crypto", "tag=usdt", "GET"},
		{"crypto.get", func(c *Client) error {
			_, err := c.Crypto.Get(ctx, "usdt-usd")
			return err
		}, "/v2/crypto/usdt-usd", "", "GET"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newRecorderClient(t, "{}")
			if err := tc.call(c); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if rec.path != tc.wantPath {
				t.Errorf("path = %q, want %q", rec.path, tc.wantPath)
			}
			if rec.query != tc.wantQuery {
				t.Errorf("query = %q, want %q", rec.query, tc.wantQuery)
			}
			if rec.method != tc.wantMeth {
				t.Errorf("method = %q, want %q", rec.method, tc.wantMeth)
			}
		})
	}
}

func TestConvertBulkPostsBody(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}
		fmt.Fprint(w, `{"count":2,"succeeded":2,"failed":0,"results":[]}`)
	})
	res, err := c.ConvertBulk(context.Background(), BulkConvertOptions{
		Conversions: []BulkConvertItemInput{
			{From: "USD", To: "ARS", Amount: 100},
			{From: "USD", To: "COP", Amount: 250},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 2 || res.Succeeded != 2 {
		t.Errorf("res = %+v", res)
	}
}

func TestInputValidation(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be reached when input validation fails")
	})
	ctx := context.Background()
	if _, err := c.Convert(ctx, ConvertOptions{From: "USD", To: "BRL", Amount: 0}); err == nil {
		t.Error("convert with Amount=0 should error")
	}
	if _, err := c.ConvertBulk(ctx, BulkConvertOptions{}); err == nil {
		t.Error("convertBulk with no items should error")
	}
	if _, err := c.Rates.Get(ctx, "", "co-trm"); err == nil {
		t.Error("empty country should error")
	}
	if _, err := c.Rates.Get(ctx, "CO", ""); err == nil {
		t.Error("empty rate id should error")
	}
	if _, err := c.Holidays.BusinessDays(ctx, "VE", BusinessDaysOptions{Days: -1}); err == nil {
		t.Error("negative business days should error")
	}
	big := make([]BulkConvertItemInput, BulkConvertMax+1)
	if _, err := c.ConvertBulk(ctx, BulkConvertOptions{Conversions: big}); err == nil {
		t.Error("too many conversions should error")
	}
}

func TestDecodeConvertResponse(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"from":{"currency":"USD","amount":"100"},"to":{"currency":"BRL","amount":"550.1"},"rate":"5.501","via":"br-official"}`)
	})
	res, err := c.Convert(context.Background(), ConvertOptions{From: "USD", To: "BRL", Amount: 100})
	if err != nil {
		t.Fatal(err)
	}
	if res.To.Amount.String() != "550.1" || res.Rate.String() != "5.501" || res.Via != "br-official" {
		t.Errorf("res = %+v", res)
	}
}
