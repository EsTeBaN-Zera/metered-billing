package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"metered-billing/internal/controllers"
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
	addr := os.Getenv(domain.EnvHTTPAddr)
	if addr == "" {
		addr = domain.DefaultHTTPAddr
	}
	pepper := os.Getenv(domain.EnvKeyPepper)
	if pepper == "" {
		log.Fatal(domain.MsgKeyPepperRequired)
	}
	apiVersion := os.Getenv(domain.EnvAPIVersion)
	if apiVersion == "" {
		apiVersion = domain.DefaultAPIVersion
	}
	opsToken := os.Getenv(domain.EnvOpsToken)
	if opsToken == "" {
		log.Fatal(domain.MsgOpsTokenRequired)
	}
	webhookSecret := os.Getenv(domain.EnvWebhookSecret)
	if webhookSecret == "" {
		log.Fatal(domain.MsgWebhookSecretRequired)
	}
	origins := os.Getenv(domain.EnvCORSOrigins)
	if origins == "" {
		origins = domain.DefaultCORSOrigin
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
		ReadHeaderTimeout: domain.ReadHeaderTimeout,
	}

	go func() {
		log.Printf(domain.MsgAPIListening, addr, apiVersion)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), domain.ShutdownTimeout)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}
