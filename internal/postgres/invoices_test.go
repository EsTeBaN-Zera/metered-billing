package postgres

import (
	"context"
	"testing"
	"time"

	"metered-billing/internal/models"
	"metered-billing/internal/services"
)

func TestIssuePeriod_tiersAndIdempotent(t *testing.T) {
	store := testStore(t)
	customerID, keyID := seedKey(t, store.Pool)
	ctx := context.Background()
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	ts := time.Date(2026, 3, 15, 14, 3, 0, 0, time.UTC)

	_, err := store.InsertBatch(ctx, customerID, keyID, []models.Event{{
		RequestID: "inv-" + keyID,
		Endpoint:  "/translate",
		Units:     150000,
		Timestamp: ts,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&services.HourService{Windows: store}).Run(ctx, 5000); err != nil {
		t.Fatal(err)
	}

	invoices := &services.InvoiceService{Store: store, Clock: services.RealClock{}}
	if _, err := invoices.IssuePeriod(ctx, start, end); err != nil {
		t.Fatal(err)
	}

	subtotal, total, lines := loadInvoice(t, store, customerID, start)
	if subtotal != 115_000_000 || total != 115_000_000 {
		t.Fatalf("subtotal=%d total=%d", subtotal, total)
	}
	if lines != 3 {
		t.Fatalf("lines=%d want 3", lines)
	}

	if _, err := invoices.IssuePeriod(ctx, start, end); err != nil {
		t.Fatal(err)
	}
	if n := invoiceCount(t, store, customerID, start); n != 1 {
		t.Fatalf("invoices=%d want 1", n)
	}
}

func TestIssuePeriod_skipsWhileDirty(t *testing.T) {
	store := testStore(t)
	customerID, keyID := seedKey(t, store.Pool)
	ctx := context.Background()
	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	_, err := store.InsertBatch(ctx, customerID, keyID, []models.Event{{
		RequestID: "dirty-a-" + keyID,
		Endpoint:  "/translate",
		Units:     5000,
		Timestamp: time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&services.HourService{Windows: store}).Run(ctx, 5000); err != nil {
		t.Fatal(err)
	}
	_, err = store.InsertBatch(ctx, customerID, keyID, []models.Event{{
		RequestID: "dirty-b-" + keyID,
		Endpoint:  "/translate",
		Units:     20000,
		Timestamp: time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC),
	}})
	if err != nil {
		t.Fatal(err)
	}

	invoices := &services.InvoiceService{Store: store, Clock: services.RealClock{}}
	if _, err := invoices.IssuePeriod(ctx, start, end); err != nil {
		t.Fatal(err)
	}
	if invoiceCount(t, store, customerID, start) != 0 {
		t.Fatal("invoice while dirty hours remain")
	}

	if _, err := (&services.HourService{Windows: store}).Run(ctx, 5000); err != nil {
		t.Fatal(err)
	}
	if _, err := invoices.IssuePeriod(ctx, start, end); err != nil {
		t.Fatal(err)
	}
	if invoiceCount(t, store, customerID, start) != 1 {
		t.Fatal("want 1 invoice after dirty hours drain")
	}
	subtotal, _, _ := loadInvoice(t, store, customerID, start)
	if subtotal != 15_000_000 {
		t.Fatalf("subtotal=%d want 15000000 (25k units, 10k free)", subtotal)
	}
}

func loadInvoice(t *testing.T, store *Store, customerID string, start time.Time) (subtotal, total int64, lineCount int) {
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
	err = tx.QueryRow(ctx, `
		SELECT id::text, subtotal_micros, total_micros
		FROM invoices WHERE customer_id = $1 AND period_start = $2
	`, customerID, start).Scan(&id, &subtotal, &total)
	if err != nil {
		t.Fatal(err)
	}
	err = tx.QueryRow(ctx, `SELECT count(*) FROM invoice_line_items WHERE invoice_id = $1`, id).Scan(&lineCount)
	if err != nil {
		t.Fatal(err)
	}
	return subtotal, total, lineCount
}

func invoiceCount(t *testing.T, store *Store, customerID string, start time.Time) int {
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
	var n int
	err = tx.QueryRow(ctx, `
		SELECT count(*) FROM invoices WHERE customer_id = $1 AND period_start = $2
	`, customerID, start).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}
