package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"metered-billing/internal/postgres"
	"metered-billing/internal/services"
)

func TestPostEvents_replayIsOk(t *testing.T) {
	store, err := postgres.Connect(context.Background(), "postgres://app:app@localhost:5432/billing?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)

	pepper := services.PepperHash{Pepper: "dev-key-pepper-change-me"}
	plaintext, prefix, err := services.NewPlaintext()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	tx, err := store.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.is_ops', 'true', true)`); err != nil {
		t.Fatal(err)
	}
	var customerID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO customers (name, price_plan_id)
		VALUES ($1, '11111111-1111-1111-1111-111111111111')
		RETURNING id::text
	`, "http-"+prefix).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO api_keys (customer_id, name, prefix, key_hash)
		VALUES ($1, 'test', $2, $3)
	`, customerID, prefix, pepper.Sum(plaintext)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	h := (&Controller{
		Auth:     &services.AuthService{Keys: store, Hasher: pepper},
		Ingest:   &services.IngestService{Store: store},
		Usage:    &services.UsageService{Store: store},
		Invoices: &services.InvoiceService{Store: store, Clock: services.RealClock{}},
		DB:       store,
	}).Handler()

	body := []byte(`{"events":[{"request_id":"http-` + prefix + `","endpoint":"/translate","units":3,"timestamp":"2026-03-15T14:03:00Z"}]}`)
	post := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+plaintext)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	first := post()
	second := post()
	if first.Code != 200 || second.Code != 200 {
		t.Fatalf("status %d then %d body=%s", first.Code, second.Code, second.Body.String())
	}
	var a, b struct {
		Inserted   int `json:"inserted"`
		Duplicates int `json:"duplicates"`
	}
	_ = json.Unmarshal(first.Body.Bytes(), &a)
	_ = json.Unmarshal(second.Body.Bytes(), &b)
	if a.Inserted != 1 || b.Duplicates != 1 {
		t.Fatalf("first=%+v second=%+v", a, b)
	}
}

func TestPostEvents_wrongKey(t *testing.T) {
	store, err := postgres.Connect(context.Background(), "postgres://app:app@localhost:5432/billing?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)

	h := (&Controller{
		Auth:   &services.AuthService{Keys: store, Hasher: services.PepperHash{Pepper: "dev-key-pepper-change-me"}},
		Ingest: &services.IngestService{Store: store},
		DB:     store,
	}).Handler()

	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader([]byte(`{"events":[]}`)))
	req.Header.Set("Authorization", "Bearer sk_live_nope")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
}
