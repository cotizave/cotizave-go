// Package cotizave is the official Go client for the Cotizave Exchange Rates
// API v2 — official (central-bank) and market exchange rates, currency
// conversion, historical series, bank rates and calendars across Latin
// America, North America and Europe, behind one API key.
//
// # Quick start
//
//	client, err := cotizave.New(cotizave.Options{APIKey: os.Getenv("COTIZAVE_API_KEY")})
//	if err != nil {
//		log.Fatal(err)
//	}
//	ctx := context.Background()
//
//	trm, err := client.Rates.Get(ctx, "CO", "co-trm")
//	conv, err := client.Convert(ctx, cotizave.ConvertOptions{From: "USD", To: "BRL", Amount: 100})
//
// The client reads COTIZAVE_API_KEY from the environment when Options.APIKey is
// empty, so cotizave.New(cotizave.Options{}) works with the variable set.
//
// # Design
//
// The SDK has zero third-party dependencies — it is built entirely on the Go
// standard library. Responses are returned as typed structs. Every request
// method takes a context.Context as its first argument; cancel it or give it a
// deadline to abort a call.
//
// # Security
//
// Cotizave is a server-side API. Keep your key in a server environment
// variable and never ship it in a client binary. The SDK sends the key only as
// the X-API-Key header, pins every request to the configured origin, refuses
// to follow redirects (so the key can never be forwarded elsewhere), enforces
// HTTPS, and caps the response size it will read. See the README for details.
package cotizave

// Version is the SDK version. A released git tag vX.Y.Z must match it.
const Version = "0.1.0"

// userAgentPrefix is the base User-Agent the SDK sends.
const userAgentPrefix = "cotizave-sdk-go/" + Version
