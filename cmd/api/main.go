package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"metered-billing/internal/controllers"
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
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	pepper := os.Getenv("KEY_PEPPER")
	if pepper == "" {
		log.Fatal("KEY_PEPPER is required")
	}
	apiVersion := os.Getenv("API_VERSION")
	if apiVersion == "" {
		apiVersion = "v1"
	}
	opsToken := os.Getenv("OPS_TOKEN")
	if opsToken == "" {
		log.Fatal("OPS_TOKEN is required")
	}
	webhookSecret := os.Getenv("WEBHOOK_SECRET")
	if webhookSecret == "" {
		log.Fatal("WEBHOOK_SECRET is required")
	}
	origins := os.Getenv("CORS_ORIGINS")
	if origins == "" {
		origins = "http://localhost:5173"
	}

	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	hash := services.PepperHash{Pepper: pepper}
	ctrl := &controllers.Controller{
		Auth:         &services.AuthService{Keys: store, Hasher: hash},
		Ingest:       &services.IngestService{Store: store},
		Usage:        &services.UsageService{Store: store},
		Invoices:     &services.InvoiceService{Store: store, Clock: services.RealClock{}},
		Ops:          &services.OpsService{Store: store},
		DB:           store,
		APIVersion:   apiVersion,
		OpsToken:     opsToken,
		WebhookCheck: services.HMACVerifier{Secret: webhookSecret},
	}

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           controllers.CORS(strings.Split(origins, ","), ctrl.Handler()),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("api listening on %s (%s)", addr, apiVersion)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}
