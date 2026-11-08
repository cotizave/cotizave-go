// Command example demonstrates the Cotizave Go SDK end to end.
//
// Run it with your key in the environment:
//
//	COTIZAVE_API_KEY=ctz_... go run ./examples
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	cotizave "github.com/cotizave/cotizave-go"
)

func main() {
	client, err := cotizave.New(cotizave.Options{
		APIKey:    os.Getenv("COTIZAVE_API_KEY"),
		UserAgent: "cotizave-go-example/1.0",
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// A single rate by canonical id.
	trm, err := client.Rates.Get(ctx, "CO", "co-trm")
	if err != nil {
		fatal(err)
	}
	fmt.Printf("TRM (co-trm): %s %s/%s updated %s\n", trm.Mid, trm.Base, trm.Quote, trm.UpdatedAt)

	// Convert — resolved on Cotizave.
	conv, err := client.Convert(ctx, cotizave.ConvertOptions{From: "USD", To: "BRL", Amount: 100})
	if err != nil {
		fatal(err)
	}
	fmt.Printf("%s %s = %s %s (via %s)\n", conv.From.Amount, conv.From.Currency, conv.To.Amount, conv.To.Currency, conv.Via)

	// The global officials feed, restricted to a few currencies.
	global, err := client.Rates.Global(ctx, &cotizave.GlobalRatesOptions{Symbols: []string{"BRL", "ARS", "COP"}})
	if err != nil {
		fatal(err)
	}
	for _, r := range global.Rates {
		fmt.Printf("  %s: %s %s\n", r.Country, r.Mid, r.Quote)
	}
}

func fatal(err error) {
	var e *cotizave.Error
	if errors.As(err, &e) {
		log.Fatalf("cotizave error %d %s: %s", e.Status, e.Code, e.Message)
	}
	log.Fatal(err)
}
