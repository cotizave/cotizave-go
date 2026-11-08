package cotizave_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	cotizave "github.com/cotizave/cotizave-go"
)

func ExampleNew() {
	client, err := cotizave.New(cotizave.Options{APIKey: os.Getenv("COTIZAVE_API_KEY")})
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	trm, err := client.Rates.Get(ctx, "CO", "co-trm")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(trm.ID, trm.Mid)
}

func ExampleClient_Convert() {
	client, _ := cotizave.New(cotizave.Options{APIKey: os.Getenv("COTIZAVE_API_KEY")})
	conv, err := client.Convert(context.Background(), cotizave.ConvertOptions{
		From:   "USD",
		To:     "BRL",
		Amount: 100,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s %s = %s %s\n", conv.From.Amount, conv.From.Currency, conv.To.Amount, conv.To.Currency)
}

func ExampleRatesResource_List() {
	client, _ := cotizave.New(cotizave.Options{APIKey: os.Getenv("COTIZAVE_API_KEY")})
	res, err := client.Rates.List(context.Background(), "AR", nil)
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range res.Rates {
		fmt.Println(r.ID, r.Mid)
	}
}

func ExampleError_handling() {
	client, _ := cotizave.New(cotizave.Options{APIKey: os.Getenv("COTIZAVE_API_KEY")})

	// Per-call timeout via context.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := client.Rates.Get(ctx, "CO", "co-trm")
	switch {
	case cotizave.IsNotFound(err):
		fmt.Println("rate not found")
	case cotizave.IsRateLimited(err):
		var rle *cotizave.RateLimitError
		errors.As(err, &rle)
		if rle.IsQuotaExhausted() {
			fmt.Println("monthly quota exhausted")
		} else {
			fmt.Println("slow down for", rle.RetryAfter(), "seconds")
		}
	case err != nil:
		fmt.Println("request failed:", err)
	}
}
