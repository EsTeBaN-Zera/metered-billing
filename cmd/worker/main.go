package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"metered-billing/internal/postgres"
	"metered-billing/internal/services"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	hours := &services.HourService{Windows: store}
	invoices := &services.InvoiceService{Store: store, Clock: services.RealClock{}}
	run := func() {
		n, err := hours.Run(ctx, 100)
		if err != nil {
			log.Printf("hour job: %v", err)
			return
		}
		if n > 0 {
			log.Printf("hour job processed %d dirty hours", n)
		}
		issued, err := invoices.IssuePreviousMonth(ctx)
		if err != nil {
			log.Printf("invoice job: %v", err)
			return
		}
		if issued > 0 {
			log.Printf("invoice job issued %d", issued)
		}
	}

	log.Print("worker up")
	run()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Print("worker stopping")
			return
		case <-ticker.C:
			run()
		}
	}
}
