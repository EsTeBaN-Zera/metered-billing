package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"metered-billing/internal/domain"
	"metered-billing/internal/postgres"
	"metered-billing/internal/services"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	databaseURL := os.Getenv(domain.EnvDatabaseURL)
	if databaseURL == "" {
		log.Fatal(domain.MsgDatabaseURLRequired)
	}

	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	hours := &services.HourService{Windows: store}
	invoices := &services.InvoiceService{Store: store, Clock: services.RealClock{}}
	run := func() {
		n, err := hours.Run(ctx, domain.DefaultHourJobLimit)
		if err != nil {
			log.Printf(domain.MsgHourJobErr, err)
			return
		}
		if n > 0 {
			log.Printf(domain.MsgHourJobProcessed, n)
		}
		issued, err := invoices.IssuePreviousMonth(ctx)
		if err != nil {
			log.Printf(domain.MsgInvoiceJobErr, err)
			return
		}
		if issued > 0 {
			log.Printf(domain.MsgInvoiceJobIssued, issued)
		}
	}

	log.Print(domain.MsgWorkerUp)
	run()
	ticker := time.NewTicker(domain.WorkerTick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Print(domain.MsgWorkerStopping)
			return
		case <-ticker.C:
			run()
		}
	}
}
