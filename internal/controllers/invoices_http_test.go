package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"metered-billing/internal/models"
	"metered-billing/internal/postgres"
	"metered-billing/internal/services"
)

func TestGetInvoice_otherCustomer404(t *testing.T) {
	store, err := postgres.Connect(context.Background(), "postgres://app:app@localhost:5432/billing?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	ctx := context.Background()
	pepper := services.PepperHash{Pepper: "dev-key-pepper-change-me"}

	ownerPlain, ownerPrefix, _ := services.NewPlaintext()
	otherPlain, otherPrefix, _ := services.NewPlaintext()
	ownerID, ownerKey := insertCustomerKey(t, store, "own-"+ownerPrefix, ownerPrefix, pepper.Sum(ownerPlain))
	_, _ = insertCustomerKey(t, store, "oth-"+otherPrefix, otherPrefix, pepper.Sum(otherPlain))

	_, err = store.InsertBatch(ctx, ownerID, ownerKey, []models.Event{{
		RequestID: "iso-" + ownerKey,
		Endpoint:  "/x",
		Units:     150000,
		Timestamp: time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
	}})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = (&services.HourService{Windows: store}).Run(ctx, 5000)
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	_, err = (&services.InvoiceService{Store: store, Clock: services.RealClock{}}).IssuePeriod(ctx, start, end)
	if err != nil {
		t.Fatal(err)
	}
	id := invoiceIDFor(t, store, ownerID, start)

	h := (&Controller{
		Auth:     &services.AuthService{Keys: store, Hasher: pepper},
		Ingest:   &services.IngestService{Store: store},
		Usage:    &services.UsageService{Store: store},
		Invoices: &services.InvoiceService{Store: store, Clock: services.RealClock{}},
		DB:       store,
	}).Handler()

	req := httptest.NewRequest(http.MethodGet, "/v1/invoices/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+otherPlain)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d body=%s want 404", rec.Code, rec.Body.String())
	}
}

func TestGetUsage_returnsWindow(t *testing.T) {
	store, err := postgres.Connect(context.Background(), "postgres://app:app@localhost:5432/billing?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	ctx := context.Background()
	pepper := services.PepperHash{Pepper: "dev-key-pepper-change-me"}
	plain, prefix, _ := services.NewPlaintext()
	cid, kid := insertCustomerKey(t, store, "use-"+prefix, prefix, pepper.Sum(plain))

	_, err = store.InsertBatch(ctx, cid, kid, []models.Event{{
		RequestID: "use-" + kid,
		Endpoint:  "/x",
		Units:     7,
		Timestamp: time.Date(2026, 5, 2, 8, 15, 0, 0, time.UTC),
	}})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = (&services.HourService{Windows: store}).Run(ctx, 5000)

	h := (&Controller{
		Auth:     &services.AuthService{Keys: store, Hasher: pepper},
		Ingest:   &services.IngestService{Store: store},
		Usage:    &services.UsageService{Store: store},
		Invoices: &services.InvoiceService{Store: store, Clock: services.RealClock{}},
		DB:       store,
	}).Handler()

	req := httptest.NewRequest(http.MethodGet, "/v1/usage?from=2026-05-01T00:00:00Z&to=2026-06-01T00:00:00Z", nil)
	req.Header.Set("Authorization", "Bearer "+plain)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() < 10 {
		t.Fatal("empty body")
	}
}

func insertCustomerKey(t *testing.T, store *postgres.Store, name, prefix, hash string) (customerID, keyID string) {
	t.Helper()
	ctx := context.Background()
	tx, err := store.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.is_ops', 'true', true)`); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO customers (name, price_plan_id)
		VALUES ($1, '11111111-1111-1111-1111-111111111111')
		RETURNING id::text
	`, name).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO api_keys (customer_id, name, prefix, key_hash)
		VALUES ($1, 'k', $2, $3) RETURNING id::text
	`, customerID, prefix, hash).Scan(&keyID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return customerID, keyID
}

func invoiceIDFor(t *testing.T, store *postgres.Store, customerID string, start time.Time) string {
	t.Helper()
	ctx := context.Background()
	tx, err := store.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.is_ops', 'true', true)`); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := tx.QueryRow(ctx, `
		SELECT id::text FROM invoices WHERE customer_id = $1 AND period_start = $2
	`, customerID, start).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
