package cotizave

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client is the Cotizave API v2 client. Create one with New and share it; it is
// safe for concurrent use by multiple goroutines.
type Client struct {
	t   *transport
	cfg Config

	// Rates reads per-country rates, a past board, and the global officials feed.
	Rates *RatesResource
	// History reads point-in-time, daily and raw history, plus coverage limits.
	History *HistoryResource
	// Countries is the published countries catalog (Pro+).
	Countries *CountriesResource
	// Currencies is the currencies catalog (Pro+).
	Currencies *CurrenciesResource
	// Banks is the bank identities directory.
	Banks *BanksResource
	// Issuers is the official-rate issuers directory.
	Issuers *IssuersResource
	// Holidays is holiday calendars and business-day math.
	Holidays *HolidaysResource
	// Crypto is stablecoin references priced in USD.
	Crypto *CryptoResource

	conversion *conversionResource
	fx         *fxToolsResource
}

// New creates a client. If Options.APIKey is empty, the COTIZAVE_API_KEY
// environment variable is used. It returns an error for misconfiguration.
func New(opts Options) (*Client, error) {
	cfg, err := resolveConfig(opts)
	if err != nil {
		return nil, err
	}
	t := newTransport(cfg, opts.HTTPClient)
	c := &Client{
		t: t,
		cfg: Config{
			BaseURL:          cfg.baseURL,
			Timeout:          cfg.timeout,
			HistoryTimeout:   cfg.historyTimeout,
			MaxRetries:       cfg.maxRetries,
			MaxRetryDelay:    cfg.maxRetryDelay,
			MaxResponseBytes: cfg.maxResponseBytes,
			UserAgent:        cfg.userAgent,
			Language:         cfg.language,
		},
	}
	c.Rates = &RatesResource{t: t}
	c.History = &HistoryResource{t: t}
	c.Countries = &CountriesResource{t: t}
	c.Currencies = &CurrenciesResource{t: t}
	c.Banks = &BanksResource{t: t}
	c.Issuers = &IssuersResource{t: t}
	c.Holidays = &HolidaysResource{t: t}
	c.Crypto = &CryptoResource{t: t}
	c.conversion = &conversionResource{t: t}
	c.fx = &fxToolsResource{t: t}
	return c, nil
}

// Config returns the resolved configuration. It never includes the API key, so
// it is safe to log.
func (c *Client) Config() Config { return c.cfg }

// String returns a safe description that never includes the API key.
func (c *Client) String() string {
	return fmt.Sprintf("cotizave.Client{BaseURL:%s, Version:%s}", c.cfg.BaseURL, Version)
}

// GoString returns a safe description that never includes the API key.
func (c *Client) GoString() string { return c.String() }

// Convert converts a single amount.
func (c *Client) Convert(ctx context.Context, opts ConvertOptions, cfg ...RequestConfig) (*ConvertResponse, error) {
	return c.conversion.convert(ctx, opts, firstConfig(cfg))
}

// ConvertBulk converts up to 50 items in one call.
func (c *Client) ConvertBulk(ctx context.Context, opts BulkConvertOptions, cfg ...RequestConfig) (*BulkConvertResponse, error) {
	return c.conversion.convertBulk(ctx, opts, firstConfig(cfg))
}

// Markets returns the country's rate catalog (no prices). Growth+.
func (c *Client) Markets(ctx context.Context, country string, cfg ...RequestConfig) (*MarketsCatalog, error) {
	return c.fx.markets(ctx, country, firstConfig(cfg))
}

// Spread returns the spread of the official rate against another market. Growth+.
func (c *Client) Spread(ctx context.Context, country string, opts *SpreadOptions, cfg ...RequestConfig) (*SpreadResponse, error) {
	return c.fx.spread(ctx, country, opts, firstConfig(cfg))
}

// Route returns the best executable rate for an amount. Pro+.
func (c *Client) Route(ctx context.Context, country string, opts RouteOptions, cfg ...RequestConfig) (*RouteResponse, error) {
	return c.fx.route(ctx, country, opts, firstConfig(cfg))
}

// BankRates returns BCV per-bank quotations (all plans).
func (c *Client) BankRates(ctx context.Context, country string, opts *BankRatesOptions, cfg ...RequestConfig) (*BankRatesResponse, error) {
	return c.fx.bankRates(ctx, country, opts, firstConfig(cfg))
}

// RawResult is the low-level result returned by Request.
type RawResult struct {
	// Body is the raw response body (empty on 304 Not Modified).
	Body []byte
	// Meta is the status, headers and parsed rate-limit metadata.
	Meta ResponseMeta
	// NotModified is true when the server answered 304 Not Modified.
	NotModified bool
}

// Decode unmarshals the JSON body into v. It is a no-op on an empty body.
func (r *RawResult) Decode(v any) error {
	if len(bytes.TrimSpace(r.Body)) == 0 {
		return nil
	}
	return json.Unmarshal(r.Body, v)
}

// RequestOptions are the options of the low-level Request.
type RequestOptions struct {
	Method           string // defaults to GET
	Path             string // e.g. "/v2/fx/ar/rates"
	Query            url.Values
	Body             any // JSON body (POST only)
	Headers          map[string]string
	Timeout          time.Duration
	MaxRetries       *int
	Language         string
	IfNoneMatch      string // sends If-None-Match for a conditional GET
	Accept           string // Accept override
	MaxResponseBytes int64
}

// Request performs a low-level request with full response metadata. It supports
// conditional GETs (IfNoneMatch) and non-JSON responses. Most code should use
// the typed resources instead.
func (c *Client) Request(ctx context.Context, opts RequestOptions) (*RawResult, error) {
	method := opts.Method
	if method == "" {
		method = http.MethodGet
	}
	spec := requestSpec{
		method:      method,
		path:        opts.Path,
		query:       opts.Query,
		body:        opts.Body,
		accept:      opts.Accept,
		ifNoneMatch: opts.IfNoneMatch,
		maxBytes:    opts.MaxResponseBytes,
		cfg: &RequestConfig{
			Timeout:    opts.Timeout,
			MaxRetries: opts.MaxRetries,
			Headers:    opts.Headers,
			Language:   opts.Language,
		},
	}
	raw, err := c.t.do(ctx, spec)
	if err != nil {
		return nil, err
	}
	return &RawResult{Body: raw.body, Meta: raw.meta, NotModified: raw.notModified}, nil
}

// Export fetches a resource as CSV or XML and returns the raw body text. format
// must be "csv" or "xml".
func (c *Client) Export(ctx context.Context, path, format string, query url.Values, cfg ...RequestConfig) (string, error) {
	var accept string
	switch format {
	case "csv":
		accept = "text/csv"
	case "xml":
		accept = "application/xml"
	default:
		return "", &Error{Code: "invalid_argument", Message: "export: format must be \"csv\" or \"xml\""}
	}
	if query == nil {
		query = url.Values{}
	}
	query.Set("format", format)
	spec := requestSpec{
		method: http.MethodGet,
		path:   path,
		query:  query,
		accept: accept,
		cfg:    firstConfig(cfg),
	}
	raw, err := c.t.do(ctx, spec)
	if err != nil {
		return "", err
	}
	return string(raw.body), nil
}
