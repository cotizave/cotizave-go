# Cotizave Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/cotizave/cotizave-go.svg)](https://pkg.go.dev/github.com/cotizave/cotizave-go)
[![CI](https://github.com/cotizave/cotizave-go/actions/workflows/ci.yml/badge.svg)](https://github.com/cotizave/cotizave-go/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](./LICENSE)

The official Go client for the [Cotizave](https://cotizave.com) Exchange Rates
API v2 — official (central-bank) and market exchange rates, currency
conversion, historical series, bank rates and calendars across **Latin America,
North America and Europe**, behind one API key. Includes Colombia's **TRM**,
official rates for Argentina, Brazil, Mexico, Peru and Chile, and Venezuela's
**BCV**.

- **Fully typed** responses — no `map[string]any`.
- **Zero dependencies** — standard library only.
- **Context-first** — every call takes a `context.Context`.
- **Secure by default** — HTTPS enforced, origin-pinned, refuses redirects,
  caps the response size, and sends the key only as a header.

## Install

```bash
go get github.com/cotizave/cotizave-go
```

Requires Go 1.22+.

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	cotizave "github.com/cotizave/cotizave-go"
)

func main() {
	client, err := cotizave.New(cotizave.Options{APIKey: os.Getenv("COTIZAVE_API_KEY")})
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	// A country's full rate list.
	rates, err := client.Rates.List(ctx, "CO", nil)
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range rates.Rates {
		fmt.Println(r.ID, r.Mid)
	}

	// A single rate by its canonical id.
	trm, _ := client.Rates.Get(ctx, "CO", "co-trm")
	fmt.Println("TRM:", trm.Mid)

	// Convert — the arithmetic runs on Cotizave, never in the SDK.
	conv, _ := client.Convert(ctx, cotizave.ConvertOptions{From: "USD", To: "BRL", Amount: 100})
	fmt.Printf("%s %s = %s %s\n", conv.From.Amount, conv.From.Currency, conv.To.Amount, conv.To.Currency)
}
```

If `Options.APIKey` is empty, the SDK reads `COTIZAVE_API_KEY` from the
environment, so `cotizave.New(cotizave.Options{})` works with the variable set.

## Authentication

Authenticate with your API key. Keep it **server-side** — never ship it in a
client binary or expose it to a browser. The SDK sends it only as the
`X-API-Key` header.

## What you can do

| Area | Methods |
| --- | --- |
| Rates | `Rates.List`, `Rates.Get`, `Rates.Snapshot`, `Rates.Global` |
| History | `History.Get`, `History.Daily`, `History.Stats`, `History.Raw`, `History.Iterate`, `History.Limits` |
| Conversion | `Convert`, `ConvertBulk` |
| FX tools | `Markets`, `Spread`, `Route`, `BankRates` |
| Catalogs | `Countries.*`, `Currencies.*`, `Banks.*`, `Issuers.List` |
| Calendars | `Holidays.List`, `Holidays.Check`, `Holidays.Next`, `Holidays.BusinessDays` |
| Crypto | `Crypto.List`, `Crypto.Get` |
| Low-level | `Request`, `Export` (CSV/XML) |

Monetary and rate fields use `json.Number` so no precision is lost; call
`.String()` for the exact decimal, or `.Float64()` when you want a number.

## Timeouts, retries and cancellation

Pass a `context.Context` to cancel or bound any call:

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()
rate, err := client.Rates.Get(ctx, "CO", "co-trm")
```

The client also applies its own per-request timeout (default 5s; 10s for
history reads) and retries idempotent (GET) requests on connection failures,
`5xx`, and **burst** `429`s — honoring `Retry-After`, with exponential backoff
and full jitter, bounded by a total budget. A **monthly-quota** `429` is never
retried. Tune it all via `Options` and per-call `RequestConfig`.

## Error handling

Every failure is a `*cotizave.Error` (or `*cotizave.RateLimitError` for a 429).
Use the predicates or `errors.As`:

```go
_, err := client.Rates.Get(ctx, "CO", "co-trm")
switch {
case cotizave.IsNotFound(err):
	// 404
case cotizave.IsRateLimited(err):
	var rle *cotizave.RateLimitError
	errors.As(err, &rle)
	if rle.IsQuotaExhausted() {
		// monthly quota gone — back off for a while
	} else {
		// burst — retry after rle.RetryAfter() seconds
	}
case err != nil:
	var e *cotizave.Error
	errors.As(err, &e)
	log.Printf("cotizave error %d %s: %s", e.Status, e.Code, e.Message)
}
```

| Status | Meaning |
| --- | --- |
| `400` | Invalid parameters or format |
| `401` | Missing, invalid or revoked API key |
| `402` | Feature not included in your plan |
| `403` | Account restriction or out-of-retention access |
| `404` | Resource or rate not found |
| `406` | Format cannot represent the response |
| `422` | No usable reference right now |
| `429` | Rate limited — monthly quota or per-minute velocity |
| `5xx` | Server-side error |

## Security

Cotizave is designed for server-side use. The SDK:

- sends the API key **only** as the `X-API-Key` header — never in a URL, log,
  error message, or the `Config()` / `String()` output;
- **pins** every request to the configured origin and **refuses redirects**, so
  the key can never be forwarded to another host;
- **enforces HTTPS** (an `http://` base URL requires an explicit opt-in);
- **caps the response body** it will read (default 25 MB), guarding against an
  oversized or decompression-bomb response;
- **rejects header injection** (CR/LF/control characters) in custom headers.

Found a security issue? See [SECURITY.md](./SECURITY.md).

## License

[MIT](./LICENSE)
