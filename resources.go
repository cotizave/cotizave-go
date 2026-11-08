package cotizave

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// BulkConvertMax is the maximum number of conversions accepted by a single
// bulk request.
const BulkConvertMax = 50

// --- path helpers ---

// cc normalizes and encodes an ISO 3166 alpha-2 country code (case-insensitive).
func cc(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", &Error{Code: "invalid_argument", Message: "a country code (ISO 3166 alpha-2, e.g. \"AR\") is required"}
	}
	return url.PathEscape(strings.ToLower(code)), nil
}

// seg encodes a single path segment (rate id, bank code, currency code).
func seg(value, label string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", &Error{Code: "invalid_argument", Message: fmt.Sprintf("a %s is required", label)}
	}
	return url.PathEscape(value), nil
}

func setStr(v url.Values, key, val string) {
	if val != "" {
		v.Set(key, val)
	}
}

func setIntPos(v url.Values, key string, val int) {
	if val > 0 {
		v.Set(key, strconv.Itoa(val))
	}
}

func setFloat(v url.Values, key string, val float64) {
	v.Set(key, strconv.FormatFloat(val, 'f', -1, 64))
}

func (t *transport) historyConfig(cfg *RequestConfig) *RequestConfig {
	out := RequestConfig{}
	if cfg != nil {
		out = *cfg
	}
	if out.Timeout <= 0 {
		out.Timeout = t.cfg.historyTimeout
	}
	return &out
}

// --- Rates ---

// RatesResource reads exchange rates: per-country list, single rate by id, past
// board, and the global officials feed.
type RatesResource struct{ t *transport }

// ListRatesOptions filters RatesResource.List.
type ListRatesOptions struct {
	Tag      string // id, v1 id, or alias (e.g. "bcv", "reference")
	Category string // historical v2 category filter
	Base     string // base currency (ISO 4217)
	Quote    string // quote currency (ISO 4217)
	Markup   string // user markup rule — VE and authorized plans only
}

// List lists every rate published for a country.
func (r *RatesResource) List(ctx context.Context, country string, opts *ListRatesOptions, cfg ...RequestConfig) (*RatesResponse, error) {
	code, err := cc(country)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if opts != nil {
		setStr(q, "tag", opts.Tag)
		setStr(q, "category", opts.Category)
		setStr(q, "base", opts.Base)
		setStr(q, "quote", opts.Quote)
		setStr(q, "markup", opts.Markup)
	}
	var out RatesResponse
	if err := r.t.getJSON(ctx, "/v2/fx/"+code+"/rates", q, firstConfig(cfg), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches one rate by its canonical id (e.g. "co-trm").
func (r *RatesResource) Get(ctx context.Context, country, rateID string, cfg ...RequestConfig) (*Rate, error) {
	code, err := cc(country)
	if err != nil {
		return nil, err
	}
	id, err := seg(rateID, "rate id")
	if err != nil {
		return nil, err
	}
	var out Rate
	if err := r.t.getJSON(ctx, "/v2/fx/"+code+"/rates/"+id, nil, firstConfig(cfg), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Snapshot returns the full board of a country as it closed on a past day.
func (r *RatesResource) Snapshot(ctx context.Context, country, date string, cfg ...RequestConfig) (*RatesSnapshotResponse, error) {
	code, err := cc(country)
	if err != nil {
		return nil, err
	}
	d, err := seg(date, "date")
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("date", d)
	var out RatesSnapshotResponse
	if err := r.t.getJSON(ctx, "/v2/fx/"+code+"/rates", q, r.t.historyConfig(firstConfig(cfg)), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GlobalRatesOptions filters RatesResource.Global.
type GlobalRatesOptions struct {
	Symbols []string // restrict to these quote currencies (ISO 4217)
	Base    string   // base currency (paid plans only); defaults to USD
}

// Global returns the official rate of every published country in one call.
func (r *RatesResource) Global(ctx context.Context, opts *GlobalRatesOptions, cfg ...RequestConfig) (*GlobalRatesList, error) {
	q := url.Values{}
	if opts != nil {
		if len(opts.Symbols) > 0 {
			q.Set("symbols", strings.Join(opts.Symbols, ","))
		}
		setStr(q, "base", opts.Base)
	}
	var out GlobalRatesList
	if err := r.t.getJSON(ctx, "/v2/fx/rates", q, firstConfig(cfg), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- History ---

// HistoryResource reads point-in-time, daily, and raw history plus coverage.
type HistoryResource struct{ t *transport }

func historyPath(country, rateID string) (string, error) {
	code, err := cc(country)
	if err != nil {
		return "", err
	}
	id, err := seg(rateID, "rate id")
	if err != nil {
		return "", err
	}
	return "/v2/fx/" + code + "/rates/" + id + "/history", nil
}

// HistoryPointOptions is a single-day history lookup.
type HistoryPointOptions struct {
	Country string
	RateID  string
	Date    string // the day to read (YYYY-MM-DD)
}

// Get returns the rate that ruled on a specific day.
func (r *HistoryResource) Get(ctx context.Context, opts HistoryPointOptions, cfg ...RequestConfig) (*HistoryPoint, error) {
	path, err := historyPath(opts.Country, opts.RateID)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	setStr(q, "date", opts.Date)
	var out HistoryPoint
	if err := r.t.getJSON(ctx, path, q, r.t.historyConfig(firstConfig(cfg)), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// HistoryDailyOptions is a daily series over a date range.
type HistoryDailyOptions struct {
	Country string
	RateID  string
	From    string
	To      string
}

// Daily returns a daily series between From and To.
func (r *HistoryResource) Daily(ctx context.Context, opts HistoryDailyOptions, cfg ...RequestConfig) (*HistoryCollection, error) {
	path, err := historyPath(opts.Country, opts.RateID)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("granularity", "daily")
	setStr(q, "from", opts.From)
	setStr(q, "to", opts.To)
	var out HistoryCollection
	if err := r.t.getJSON(ctx, path, q, r.t.historyConfig(firstConfig(cfg)), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// HistoryStatsOptions is a stats query over a date range.
type HistoryStatsOptions struct {
	Country       string
	RateID        string
	From          string
	To            string
	MovingAverage int // window in days (2–365); adds sma/ema
}

// Stats returns aggregates of a date range computed over the daily closes.
func (r *HistoryResource) Stats(ctx context.Context, opts HistoryStatsOptions, cfg ...RequestConfig) (*HistoryStatsResponse, error) {
	path, err := historyPath(opts.Country, opts.RateID)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("stats", "1")
	setStr(q, "from", opts.From)
	setStr(q, "to", opts.To)
	setIntPos(q, "ma", opts.MovingAverage)
	var out HistoryStatsResponse
	if err := r.t.getJSON(ctx, path, q, r.t.historyConfig(firstConfig(cfg)), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// HistoryRawOptions is one page of raw (intraday) captures.
type HistoryRawOptions struct {
	Country string
	RateID  string
	Cursor  string // opaque cursor from a previous page's NextCursor
	Limit   int    // page size (1–5000, default 1000)
}

// Raw returns one page of raw (intraday) captures. Pass the previous page's
// NextCursor to continue.
func (r *HistoryResource) Raw(ctx context.Context, opts HistoryRawOptions, cfg ...RequestConfig) (*HistoryCollection, error) {
	path, err := historyPath(opts.Country, opts.RateID)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("granularity", "raw")
	setStr(q, "cursor", opts.Cursor)
	setIntPos(q, "limit", opts.Limit)
	var out HistoryCollection
	if err := r.t.getJSON(ctx, path, q, r.t.historyConfig(firstConfig(cfg)), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// HistoryIterateOptions controls HistoryResource.Iterate.
type HistoryIterateOptions struct {
	Country  string
	RateID   string
	Limit    int // page size
	MaxPages int // hard cap on pages fetched (default 50)
}

// Iterate walks raw captures, following cursors for you. It calls fn for each
// point; return false from fn to stop. It is bounded by MaxPages (default 50)
// so it never silently downloads an entire history.
func (r *HistoryResource) Iterate(ctx context.Context, opts HistoryIterateOptions, fn func(HistoryPointRow) bool, cfg ...RequestConfig) error {
	maxPages := opts.MaxPages
	if maxPages <= 0 {
		maxPages = 50
	}
	cursor := ""
	for page := 0; page < maxPages; page++ {
		coll, err := r.Raw(ctx, HistoryRawOptions{
			Country: opts.Country,
			RateID:  opts.RateID,
			Limit:   opts.Limit,
			Cursor:  cursor,
		}, cfg...)
		if err != nil {
			return err
		}
		for _, p := range coll.Points {
			if !fn(p) {
				return nil
			}
		}
		if coll.NextCursor == "" {
			return nil
		}
		cursor = coll.NextCursor
	}
	return nil
}

// Limits returns the coverage (first/last/days) of the stored history for every
// rate the plan can see.
func (r *HistoryResource) Limits(ctx context.Context, cfg ...RequestConfig) (*HistoricalLimitsResponse, error) {
	var out HistoricalLimitsResponse
	if err := r.t.getJSON(ctx, "/v2/fx/historical-limits", nil, r.t.historyConfig(firstConfig(cfg)), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Conversion (wired onto the client) ---

type conversionResource struct{ t *transport }

// ConvertOptions is the input for a single conversion.
type ConvertOptions struct {
	From   string  // source currency (ISO 4217)
	To     string  // target currency (ISO 4217)
	Amount float64 // amount to convert (> 0)
	Via    string  // explicit v2 rate id to convert through
	Market string  // v1 local market alias for Via (Venezuela only)
	Date   string  // historical conversion date (YYYY-MM-DD)
}

func (c *conversionResource) convert(ctx context.Context, opts ConvertOptions, cfg *RequestConfig) (*ConvertResponse, error) {
	if !(opts.Amount > 0) {
		return nil, &Error{Code: "invalid_argument", Message: "convert: Amount must be greater than 0"}
	}
	q := url.Values{}
	setStr(q, "from", opts.From)
	setStr(q, "to", opts.To)
	setFloat(q, "amount", opts.Amount)
	setStr(q, "via", opts.Via)
	setStr(q, "market", opts.Market)
	setStr(q, "date", opts.Date)
	var out ConvertResponse
	if err := c.t.getJSON(ctx, "/v2/fx/convert", q, cfg, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BulkConvertItemInput is a single item inside a bulk conversion request.
type BulkConvertItemInput struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Amount float64 `json:"amount"`
	Via    string  `json:"via,omitempty"`
	Date   string  `json:"date,omitempty"`
}

// BulkConvertOptions is the input for a bulk conversion (max 50 items).
type BulkConvertOptions struct {
	Conversions []BulkConvertItemInput
}

func (c *conversionResource) convertBulk(ctx context.Context, opts BulkConvertOptions, cfg *RequestConfig) (*BulkConvertResponse, error) {
	if len(opts.Conversions) == 0 {
		return nil, &Error{Code: "invalid_argument", Message: "convertBulk: Conversions must be a non-empty slice"}
	}
	if len(opts.Conversions) > BulkConvertMax {
		return nil, &Error{Code: "invalid_argument", Message: fmt.Sprintf("convertBulk: at most %d conversions are allowed (got %d)", BulkConvertMax, len(opts.Conversions))}
	}
	body := map[string]any{"conversions": opts.Conversions}
	var out BulkConvertResponse
	if err := c.t.postJSON(ctx, "/v2/fx/convert", body, cfg, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- FX tools (wired onto the client) ---

type fxToolsResource struct{ t *transport }

// SpreadOptions filters the spread query.
type SpreadOptions struct {
	Against string // rate to compare the official against (default "ve-paralelo")
	Base    string // base currency (only "USD" is supported)
}

// RouteOptions is the input for a best-route query.
type RouteOptions struct {
	From         string  // source currency
	To           string  // target currency
	Amount       float64 // amount
	Direction    string  // "send" or "receive"
	MaxStaleness int     // reject rates staler than this many seconds
}

// BankRatesOptions filters bank-rate quotations.
type BankRatesOptions struct {
	Bank string // single SUDEBAN bank code (4 digits)
}

func (f *fxToolsResource) markets(ctx context.Context, country string, cfg *RequestConfig) (*MarketsCatalog, error) {
	code, err := cc(country)
	if err != nil {
		return nil, err
	}
	var out MarketsCatalog
	if err := f.t.getJSON(ctx, "/v2/fx/"+code+"/markets", nil, cfg, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (f *fxToolsResource) spread(ctx context.Context, country string, opts *SpreadOptions, cfg *RequestConfig) (*SpreadResponse, error) {
	code, err := cc(country)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if opts != nil {
		setStr(q, "against", opts.Against)
		setStr(q, "base", opts.Base)
	}
	var out SpreadResponse
	if err := f.t.getJSON(ctx, "/v2/fx/"+code+"/spread", q, cfg, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (f *fxToolsResource) route(ctx context.Context, country string, opts RouteOptions, cfg *RequestConfig) (*RouteResponse, error) {
	code, err := cc(country)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	setStr(q, "from", opts.From)
	setStr(q, "to", opts.To)
	setFloat(q, "amount", opts.Amount)
	setStr(q, "direction", opts.Direction)
	setIntPos(q, "max_staleness", opts.MaxStaleness)
	var out RouteResponse
	if err := f.t.getJSON(ctx, "/v2/fx/"+code+"/route", q, cfg, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (f *fxToolsResource) bankRates(ctx context.Context, country string, opts *BankRatesOptions, cfg *RequestConfig) (*BankRatesResponse, error) {
	code, err := cc(country)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if opts != nil {
		setStr(q, "bank", opts.Bank)
	}
	var out BankRatesResponse
	if err := f.t.getJSON(ctx, "/v2/fx/"+code+"/bank-rates", q, cfg, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Catalogs ---

// CountriesResource is the published countries catalog (Pro+).
type CountriesResource struct{ t *transport }

// List lists published countries.
func (r *CountriesResource) List(ctx context.Context, cfg ...RequestConfig) (*CountriesList, error) {
	var out CountriesList
	if err := r.t.getJSON(ctx, "/v2/countries", nil, firstConfig(cfg), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns one published country by code.
func (r *CountriesResource) Get(ctx context.Context, country string, cfg ...RequestConfig) (*Country, error) {
	code, err := cc(country)
	if err != nil {
		return nil, err
	}
	var out Country
	if err := r.t.getJSON(ctx, "/v2/countries/"+code, nil, firstConfig(cfg), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CurrenciesResource is the currencies catalog (Pro+).
type CurrenciesResource struct{ t *transport }

// List lists the currencies Cotizave quotes (ISO 4217).
func (r *CurrenciesResource) List(ctx context.Context, cfg ...RequestConfig) (*CurrenciesList, error) {
	var out CurrenciesList
	if err := r.t.getJSON(ctx, "/v2/currencies", nil, firstConfig(cfg), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns one currency by code (e.g. "USD").
func (r *CurrenciesResource) Get(ctx context.Context, code string, cfg ...RequestConfig) (*Currency, error) {
	c, err := seg(code, "currency code")
	if err != nil {
		return nil, err
	}
	var out Currency
	if err := r.t.getJSON(ctx, "/v2/currencies/"+c, nil, firstConfig(cfg), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BanksResource is the bank identities directory.
type BanksResource struct{ t *transport }

// BanksListOptions filters BanksResource.List.
type BanksListOptions struct {
	Q string // free-text search
}

// List lists a country's banks, with an optional free-text search.
func (r *BanksResource) List(ctx context.Context, country string, opts *BanksListOptions, cfg ...RequestConfig) (*BanksList, error) {
	code, err := cc(country)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if opts != nil {
		setStr(q, "q", opts.Q)
	}
	var out BanksList
	if err := r.t.getJSON(ctx, "/v2/banks/"+code, q, firstConfig(cfg), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns one bank by its official code (SUDEBAN in VE).
func (r *BanksResource) Get(ctx context.Context, country, code string, cfg ...RequestConfig) (*Bank, error) {
	cCode, err := cc(country)
	if err != nil {
		return nil, err
	}
	bCode, err := seg(code, "bank code")
	if err != nil {
		return nil, err
	}
	var out Bank
	if err := r.t.getJSON(ctx, "/v2/banks/"+cCode+"/"+bCode, nil, firstConfig(cfg), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// IssuersResource is the official-rate issuers directory.
type IssuersResource struct{ t *transport }

// List lists the institutions that issue official rates.
func (r *IssuersResource) List(ctx context.Context, cfg ...RequestConfig) (*IssuersResponse, error) {
	var out IssuersResponse
	if err := r.t.getJSON(ctx, "/v2/issuers", nil, firstConfig(cfg), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Holidays ---

// HolidaysResource is holiday calendars and business-day math.
type HolidaysResource struct{ t *transport }

// HolidaysListOptions filters HolidaysResource.List.
type HolidaysListOptions struct {
	Year int // defaults to the current year server-side
}

// List returns the holidays for a year.
func (r *HolidaysResource) List(ctx context.Context, country string, opts *HolidaysListOptions, cfg ...RequestConfig) (*HolidaysYear, error) {
	code, err := cc(country)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if opts != nil {
		setIntPos(q, "year", opts.Year)
	}
	var out HolidaysYear
	if err := r.t.getJSON(ctx, "/v2/holidays/"+code, q, firstConfig(cfg), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Check reports whether a given date is a holiday.
func (r *HolidaysResource) Check(ctx context.Context, country, date string, cfg ...RequestConfig) (*HolidayCheck, error) {
	code, err := cc(country)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	setStr(q, "date", date)
	var out HolidayCheck
	if err := r.t.getJSON(ctx, "/v2/holidays/"+code, q, firstConfig(cfg), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Next returns the next holiday on or after from.
func (r *HolidaysResource) Next(ctx context.Context, country, from string, cfg ...RequestConfig) (*NextHoliday, error) {
	code, err := cc(country)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	setStr(q, "from", from)
	var out NextHoliday
	if err := r.t.getJSON(ctx, "/v2/holidays/"+code, q, firstConfig(cfg), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BusinessDaysOptions is the input for BusinessDays.
type BusinessDaysOptions struct {
	Days int    // non-negative number of business days to add
	From string // start date (YYYY-MM-DD)
}

// BusinessDays returns the date reached by adding Days business days to From.
func (r *HolidaysResource) BusinessDays(ctx context.Context, country string, opts BusinessDaysOptions, cfg ...RequestConfig) (*BusinessDaysResult, error) {
	if opts.Days < 0 {
		return nil, &Error{Code: "invalid_argument", Message: "businessDays: Days must be a non-negative integer"}
	}
	code, err := cc(country)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	setStr(q, "from", opts.From)
	q.Set("days", strconv.Itoa(opts.Days))
	var out BusinessDaysResult
	if err := r.t.getJSON(ctx, "/v2/holidays/"+code+"/business-days", q, firstConfig(cfg), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Crypto ---

// CryptoResource is stablecoin references priced in USD.
type CryptoResource struct{ t *transport }

// CryptoListOptions filters CryptoResource.List.
type CryptoListOptions struct {
	Tag string // e.g. "usdt"
}

// List lists the published stablecoins.
func (r *CryptoResource) List(ctx context.Context, opts *CryptoListOptions, cfg ...RequestConfig) (*CryptoList, error) {
	q := url.Values{}
	if opts != nil {
		setStr(q, "tag", opts.Tag)
	}
	var out CryptoList
	if err := r.t.getJSON(ctx, "/v2/crypto", q, firstConfig(cfg), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns one stablecoin by its canonical id (e.g. "usdt-usd").
func (r *CryptoResource) Get(ctx context.Context, id string, cfg ...RequestConfig) (*Rate, error) {
	s, err := seg(id, "crypto id")
	if err != nil {
		return nil, err
	}
	var out Rate
	if err := r.t.getJSON(ctx, "/v2/crypto/"+s, nil, firstConfig(cfg), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
