package cotizave

import "encoding/json"

// Typed shapes of the Cotizave API v2 responses. They mirror the OpenAPI v2
// contract. Monetary and rate fields use json.Number to preserve the exact
// decimal the API sends (no float rounding). Fields that may be absent depend
// on the plan, the country and the market.

// GeoFields are country-level fields embedded in several responses (geography
// is Pro+).
type GeoFields struct {
	Country     string `json:"country,omitempty"`
	Currency    string `json:"currency,omitempty"`
	Continent   string `json:"continent,omitempty"`
	CountryName string `json:"country_name,omitempty"`
	Flag        string `json:"flag,omitempty"`
	Region      string `json:"region,omitempty"`
	RegionCode  string `json:"region_code,omitempty"`
	Timezone    string `json:"timezone,omitempty"`
}

// Rate is a canonical exchange-rate reference.
type Rate struct {
	GeoFields
	ID            string      `json:"id,omitempty"`
	Mid           json.Number `json:"mid,omitempty"`
	Type          string      `json:"type,omitempty"`
	Market        string      `json:"market,omitempty"`
	UpdatedAt     string      `json:"updated_at,omitempty"`
	Ask           json.Number `json:"ask,omitempty"`
	Bid           json.Number `json:"bid,omitempty"`
	Base          string      `json:"base,omitempty"`
	Quote         string      `json:"quote,omitempty"`
	Name          string      `json:"name,omitempty"`
	Description   string      `json:"description,omitempty"`
	Available     *bool       `json:"available,omitempty"`
	Change24hPct  json.Number `json:"change_24h_pct,omitempty"`
	EffectiveDate string      `json:"effective_date,omitempty"`
	FetchedAt     string      `json:"fetched_at,omitempty"`
	RequiredPlan  string      `json:"required_plan,omitempty"`
	Stale         *bool       `json:"stale,omitempty"`
	StaleAfterSec float64     `json:"stale_after_sec,omitempty"`
	StalenessSec  float64     `json:"staleness_sec,omitempty"`
	Tags          []string    `json:"tags,omitempty"`
	VsOfficialPct json.Number `json:"vs_official_pct,omitempty"`
}

// RateDescriptor is a restricted VE rate shown without a price.
type RateDescriptor struct {
	ID           string   `json:"id,omitempty"`
	Available    *bool    `json:"available,omitempty"`
	Base         string   `json:"base,omitempty"`
	Description  string   `json:"description,omitempty"`
	Market       string   `json:"market,omitempty"`
	Name         string   `json:"name,omitempty"`
	Quote        string   `json:"quote,omitempty"`
	RequiredPlan string   `json:"required_plan,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Type         string   `json:"type,omitempty"`
}

// RatesResponse is the response of Rates.List.
type RatesResponse struct {
	GeoFields
	Base      string         `json:"base,omitempty"`
	FetchedAt string         `json:"fetched_at,omitempty"`
	Plan      string         `json:"plan,omitempty"`
	Index     map[string]any `json:"index,omitempty"`
	Rates     []Rate         `json:"rates,omitempty"`
}

// RatesSnapshotRow is the close of one rate on a past date.
type RatesSnapshotRow struct {
	ID        string      `json:"id,omitempty"`
	Market    string      `json:"market,omitempty"`
	Mid       json.Number `json:"mid,omitempty"`
	Type      string      `json:"type,omitempty"`
	UpdatedAt string      `json:"updated_at,omitempty"`
	Ask       json.Number `json:"ask,omitempty"`
	Bid       json.Number `json:"bid,omitempty"`
}

// RatesSnapshotResponse is the full board of a country on a past date.
type RatesSnapshotResponse struct {
	GeoFields
	Base  string             `json:"base,omitempty"`
	Count int                `json:"count,omitempty"`
	Date  string             `json:"date,omitempty"`
	Rates []RatesSnapshotRow `json:"rates,omitempty"`
}

// GlobalRateRow is a single row of the global officials endpoint.
type GlobalRateRow struct {
	Country       string      `json:"country,omitempty"`
	ID            string      `json:"id,omitempty"`
	Mid           json.Number `json:"mid,omitempty"`
	Quote         string      `json:"quote,omitempty"`
	Stale         *bool       `json:"stale,omitempty"`
	StaleAfterSec float64     `json:"stale_after_sec,omitempty"`
	UpdatedAt     string      `json:"updated_at,omitempty"`
}

// GlobalRatesList is the response of Rates.Global.
type GlobalRatesList struct {
	Base  string          `json:"base,omitempty"`
	Count int             `json:"count,omitempty"`
	Date  string          `json:"date,omitempty"`
	Rates []GlobalRateRow `json:"rates,omitempty"`
}

// HistoryPointRow is one point inside a history collection.
type HistoryPointRow struct {
	Date       string      `json:"date,omitempty"`
	Mid        json.Number `json:"mid,omitempty"`
	FetchedAt  string      `json:"fetched_at,omitempty"`
	UpdatedAt  string      `json:"updated_at,omitempty"`
	Ask        json.Number `json:"ask,omitempty"`
	Bid        json.Number `json:"bid,omitempty"`
	CapturedAt string      `json:"captured_at,omitempty"`
	Rate       json.Number `json:"rate,omitempty"`
}

// HistoryPoint is one historical data point (History.Get with a date).
type HistoryPoint struct {
	ID         string      `json:"id,omitempty"`
	Base       string      `json:"base,omitempty"`
	Quote      string      `json:"quote,omitempty"`
	Market     string      `json:"market,omitempty"`
	Date       string      `json:"date,omitempty"`
	Mid        json.Number `json:"mid,omitempty"`
	FetchedAt  string      `json:"fetched_at,omitempty"`
	UpdatedAt  string      `json:"updated_at,omitempty"`
	Ask        json.Number `json:"ask,omitempty"`
	Bid        json.Number `json:"bid,omitempty"`
	CapturedAt string      `json:"captured_at,omitempty"`
	Rate       json.Number `json:"rate,omitempty"`
}

// HistoryCollection is a daily or raw history series.
type HistoryCollection struct {
	Base        string            `json:"base,omitempty"`
	Count       int               `json:"count,omitempty"`
	From        string            `json:"from,omitempty"`
	To          string            `json:"to,omitempty"`
	Granularity string            `json:"granularity,omitempty"`
	ID          string            `json:"id,omitempty"`
	Market      string            `json:"market,omitempty"`
	Quote       string            `json:"quote,omitempty"`
	NextCursor  string            `json:"next_cursor,omitempty"`
	Points      []HistoryPointRow `json:"points,omitempty"`
	Truncated   *bool             `json:"truncated,omitempty"`
}

// HistoryStatsPoint is a dated value inside history stats.
type HistoryStatsPoint struct {
	Date string      `json:"date,omitempty"`
	Mid  json.Number `json:"mid,omitempty"`
}

// HistoryStats are aggregates of a period.
type HistoryStats struct {
	Avg       json.Number       `json:"avg,omitempty"`
	Change    json.Number       `json:"change,omitempty"`
	ChangePct json.Number       `json:"change_pct,omitempty"`
	Count     int               `json:"count,omitempty"`
	First     HistoryStatsPoint `json:"first,omitempty"`
	Last      HistoryStatsPoint `json:"last,omitempty"`
	Max       json.Number       `json:"max,omitempty"`
	Min       json.Number       `json:"min,omitempty"`
	Stddev    json.Number       `json:"stddev,omitempty"`
	Ema       json.Number       `json:"ema,omitempty"`
	Sma       json.Number       `json:"sma,omitempty"`
	Period    int               `json:"period,omitempty"`
	VsEmaPct  json.Number       `json:"vs_ema_pct,omitempty"`
	VsSmaPct  json.Number       `json:"vs_sma_pct,omitempty"`
}

// HistoryStatsResponse is the response of History.Stats.
type HistoryStatsResponse struct {
	Base        string       `json:"base,omitempty"`
	From        string       `json:"from,omitempty"`
	To          string       `json:"to,omitempty"`
	Granularity string       `json:"granularity,omitempty"`
	ID          string       `json:"id,omitempty"`
	Market      string       `json:"market,omitempty"`
	Quote       string       `json:"quote,omitempty"`
	Stats       HistoryStats `json:"stats,omitempty"`
}

// HistoricalLimitRow is the coverage window of a rate's stored history.
type HistoricalLimitRow struct {
	Base    string `json:"base,omitempty"`
	Country string `json:"country,omitempty"`
	Days    int    `json:"days,omitempty"`
	First   string `json:"first,omitempty"`
	ID      string `json:"id,omitempty"`
	Last    string `json:"last,omitempty"`
	Quote   string `json:"quote,omitempty"`
}

// HistoricalLimitsResponse is the response of History.Limits.
type HistoricalLimitsResponse struct {
	Base   string               `json:"base,omitempty"`
	Count  int                  `json:"count,omitempty"`
	Limits []HistoricalLimitRow `json:"limits,omitempty"`
}

// Amount is a converted amount.
type Amount struct {
	Amount   json.Number `json:"amount,omitempty"`
	Currency string      `json:"currency,omitempty"`
}

// ConvertResponse is the response of a single Convert.
type ConvertResponse struct {
	From          Amount      `json:"from,omitempty"`
	To            Amount      `json:"to,omitempty"`
	Rate          json.Number `json:"rate,omitempty"`
	Market        string      `json:"market,omitempty"`
	Via           string      `json:"via,omitempty"`
	Type          string      `json:"type,omitempty"`
	Stale         *bool       `json:"stale,omitempty"`
	StalenessSec  float64     `json:"staleness_sec,omitempty"`
	UpdatedAt     string      `json:"updated_at,omitempty"`
	Date          string      `json:"date,omitempty"`
	EffectiveDate string      `json:"effective_date,omitempty"`
}

// BulkConvertItem is one entry of a bulk conversion result.
type BulkConvertItem struct {
	Index      int              `json:"index"`
	Conversion *ConvertResponse `json:"conversion,omitempty"`
	Error      *APIErrorBody    `json:"error,omitempty"`
}

// BulkConvertResponse is the response of ConvertBulk.
type BulkConvertResponse struct {
	Count     int               `json:"count,omitempty"`
	Succeeded int               `json:"succeeded,omitempty"`
	Failed    int               `json:"failed,omitempty"`
	Results   []BulkConvertItem `json:"results,omitempty"`
}

// APIErrorBody is the raw error body the API returns inside a bulk result item.
type APIErrorBody struct {
	Code    string         `json:"code,omitempty"`
	Message string         `json:"message,omitempty"`
	Field   string         `json:"field,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// ConversionDirection is a from/to pair in a market's catalog entry.
type ConversionDirection struct {
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

// MarketEntry is a catalog entry from Markets.
type MarketEntry struct {
	ID                   string                `json:"id,omitempty"`
	Name                 string                `json:"name,omitempty"`
	Market               string                `json:"market,omitempty"`
	Base                 string                `json:"base,omitempty"`
	Quote                string                `json:"quote,omitempty"`
	Type                 string                `json:"type,omitempty"`
	Available            *bool                 `json:"available,omitempty"`
	RequiredPlan         string                `json:"required_plan,omitempty"`
	StaleAfterSec        float64               `json:"stale_after_sec,omitempty"`
	Tags                 []string              `json:"tags,omitempty"`
	Aliases              []string              `json:"aliases,omitempty"`
	ConversionDirections []ConversionDirection `json:"conversion_directions,omitempty"`
}

// MarketsCatalog is the response of Markets.
type MarketsCatalog struct {
	Country string        `json:"country,omitempty"`
	Markets []MarketEntry `json:"markets,omitempty"`
}

// SpreadQuote is a quote inside a spread response.
type SpreadQuote struct {
	ID        string      `json:"id,omitempty"`
	Market    string      `json:"market,omitempty"`
	UpdatedAt string      `json:"updated_at,omitempty"`
	Value     json.Number `json:"value,omitempty"`
}

// SpreadDelta is the computed spread between two quotes.
type SpreadDelta struct {
	Absolute   json.Number `json:"absolute,omitempty"`
	Direction  string      `json:"direction,omitempty"`
	Percentage json.Number `json:"percentage,omitempty"`
}

// SpreadResponse is the response of Spread.
type SpreadResponse struct {
	Country   string      `json:"country,omitempty"`
	FetchedAt string      `json:"fetched_at,omitempty"`
	Reference SpreadQuote `json:"reference,omitempty"`
	Compared  SpreadQuote `json:"compared,omitempty"`
	Spread    SpreadDelta `json:"spread,omitempty"`
}

// RouteOption is a single executable option inside a route response.
type RouteOption struct {
	ID           string      `json:"id,omitempty"`
	Market       string      `json:"market,omitempty"`
	Amount       json.Number `json:"amount,omitempty"`
	Rate         json.Number `json:"rate,omitempty"`
	Type         string      `json:"type,omitempty"`
	StalenessSec float64     `json:"staleness_sec,omitempty"`
	UpdatedAt    string      `json:"updated_at,omitempty"`
}

// RouteSkipped is a route option the backend skipped, with a reason.
type RouteSkipped struct {
	ID     string `json:"id,omitempty"`
	Market string `json:"market,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// RouteResponse is the response of Route.
type RouteResponse struct {
	Country      string         `json:"country,omitempty"`
	Direction    string         `json:"direction,omitempty"`
	FetchedAt    string         `json:"fetched_at,omitempty"`
	From         Amount         `json:"from,omitempty"`
	Best         RouteOption    `json:"best,omitempty"`
	Alternatives []RouteOption  `json:"alternatives,omitempty"`
	Skipped      []RouteSkipped `json:"skipped,omitempty"`
}

// BankRef identifies a bank inside a bank-rate quotation.
type BankRef struct {
	Code string `json:"code,omitempty"`
	Name string `json:"name,omitempty"`
}

// BankRate is a single BCV-published bank quotation.
type BankRate struct {
	Bank          BankRef     `json:"bank,omitempty"`
	Base          string      `json:"base,omitempty"`
	Quote         string      `json:"quote,omitempty"`
	Mid           json.Number `json:"mid,omitempty"`
	Ask           json.Number `json:"ask,omitempty"`
	Bid           json.Number `json:"bid,omitempty"`
	DaysBehind    int         `json:"days_behind,omitempty"`
	EffectiveDate string      `json:"effective_date,omitempty"`
	Stale         *bool       `json:"stale,omitempty"`
	StaleAfterSec float64     `json:"stale_after_sec,omitempty"`
	StalenessSec  float64     `json:"staleness_sec,omitempty"`
	UpdatedAt     string      `json:"updated_at,omitempty"`
	VsOfficialPct json.Number `json:"vs_official_pct,omitempty"`
}

// BankRatesResponse is the response of BankRates.
type BankRatesResponse struct {
	Country       string     `json:"country,omitempty"`
	Currency      string     `json:"currency,omitempty"`
	IndicatorDate string     `json:"indicator_date,omitempty"`
	Banks         []BankRate `json:"banks,omitempty"`
}

// Country is a country catalog entry.
type Country struct {
	Code       string   `json:"code,omitempty"`
	Continent  string   `json:"continent,omitempty"`
	Currency   string   `json:"currency,omitempty"`
	Flag       string   `json:"flag,omitempty"`
	Name       string   `json:"name,omitempty"`
	Rates      []string `json:"rates,omitempty"`
	Region     string   `json:"region,omitempty"`
	RegionCode string   `json:"region_code,omitempty"`
	Timezone   string   `json:"timezone,omitempty"`
}

// CountriesList is the response of Countries.List.
type CountriesList struct {
	Countries []Country `json:"countries,omitempty"`
}

// Currency is a currency catalog entry (ISO 4217).
type Currency struct {
	Code        string `json:"code,omitempty"`
	MinorUnit   int    `json:"minor_unit,omitempty"`
	Name        string `json:"name,omitempty"`
	NumericCode string `json:"numeric_code,omitempty"`
	Symbol      string `json:"symbol,omitempty"`
}

// CurrenciesList is the response of Currencies.List.
type CurrenciesList struct {
	Currencies []Currency `json:"currencies,omitempty"`
}

// Bank is a bank identity entry.
type Bank struct {
	BankRates string `json:"bank_rates,omitempty"`
	Code      string `json:"code,omitempty"`
	Country   string `json:"country,omitempty"`
	Name      string `json:"name,omitempty"`
	Type      string `json:"type,omitempty"`
}

// BanksList is the response of Banks.List.
type BanksList struct {
	Banks []Bank `json:"banks,omitempty"`
}

// IssuerJurisdiction is an issuer's jurisdiction.
type IssuerJurisdiction struct {
	Countries []string `json:"countries,omitempty"`
	Currency  string   `json:"currency,omitempty"`
}

// IssuerLinks are an issuer's links.
type IssuerLinks struct {
	Website string `json:"website,omitempty"`
}

// IssuerPublishes describes a rate an issuer publishes.
type IssuerPublishes struct {
	Label     string `json:"label,omitempty"`
	RateID    string `json:"rate_id,omitempty"`
	Pair      string `json:"pair,omitempty"`
	Schedule  string `json:"schedule,omitempty"`
	Source    string `json:"source,omitempty"`
	Status    string `json:"status,omitempty"`
	ValueDate string `json:"value_date,omitempty"`
}

// Issuer is an official-rate issuing institution.
type Issuer struct {
	ID           string             `json:"id,omitempty"`
	Name         string             `json:"name,omitempty"`
	ShortName    string             `json:"short_name,omitempty"`
	Kind         string             `json:"kind,omitempty"`
	Jurisdiction IssuerJurisdiction `json:"jurisdiction,omitempty"`
	Links        IssuerLinks        `json:"links,omitempty"`
	Publishes    []IssuerPublishes  `json:"publishes,omitempty"`
	Timezone     string             `json:"timezone,omitempty"`
}

// IssuersSummary summarizes the issuers directory.
type IssuersSummary struct {
	Countries int `json:"countries,omitempty"`
	Issuers   int `json:"issuers,omitempty"`
}

// IssuerWithout is a rate without a known issuer.
type IssuerWithout struct {
	Country string `json:"country,omitempty"`
	RateID  string `json:"rate_id,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Status  string `json:"status,omitempty"`
}

// IssuersResponse is the response of Issuers.List.
type IssuersResponse struct {
	Issuers       []Issuer        `json:"issuers,omitempty"`
	Summary       IssuersSummary  `json:"summary,omitempty"`
	WithoutIssuer []IssuerWithout `json:"without_issuer,omitempty"`
}

// Holiday is a public holiday.
type Holiday struct {
	Date      string   `json:"date,omitempty"`
	Name      string   `json:"name,omitempty"`
	LocalName string   `json:"local_name,omitempty"`
	Fixed     *bool    `json:"fixed,omitempty"`
	Global    *bool    `json:"global,omitempty"`
	Type      string   `json:"type,omitempty"`
	Types     []string `json:"types,omitempty"`
}

// HolidaysYear is the year-mode holidays response.
type HolidaysYear struct {
	Country  string    `json:"country,omitempty"`
	Year     int       `json:"year,omitempty"`
	Holidays []Holiday `json:"holidays,omitempty"`
}

// HolidayCheck is the is-holiday response.
type HolidayCheck struct {
	Country   string   `json:"country,omitempty"`
	Date      string   `json:"date,omitempty"`
	IsHoliday *bool    `json:"is_holiday,omitempty"`
	Holiday   *Holiday `json:"holiday,omitempty"`
}

// NextHoliday is the next-holiday response.
type NextHoliday struct {
	Country   string  `json:"country,omitempty"`
	From      string  `json:"from,omitempty"`
	DaysUntil int     `json:"days_until,omitempty"`
	Holiday   Holiday `json:"holiday,omitempty"`
}

// BusinessDaysResult is the business-days response.
type BusinessDaysResult struct {
	Country string `json:"country,omitempty"`
	From    string `json:"from,omitempty"`
	Days    int    `json:"days,omitempty"`
	Result  string `json:"result,omitempty"`
}

// CryptoList is the stablecoin feed.
type CryptoList struct {
	Base      string `json:"base,omitempty"`
	FetchedAt string `json:"fetched_at,omitempty"`
	Plan      string `json:"plan,omitempty"`
	Rates     []Rate `json:"rates,omitempty"`
}
