package postgres

import (
	"context"
	"testing"
	"time"

	"metered-billing/internal/models"
	"metered-billing/internal/services"
)

func TestIssueCredit_sameKeyOnce(t *testing.T) {
	store := testStore(t)
	customerID, _ := seedKey(t, store.Pool)
	ctx := context.Background()
	ops := &services.OpsService{Store: store}

	in := models.CreditIssue{
		CustomerID:     customerID,
		AmountMicros:   20_000_000,
		Reason:         "goodwill",
		Actor:          "ops",
		IdempotencyKey: "cred-" + customerID,
	}
	a, created1, err := ops.IssueCredit(ctx, in)
	if err != nil || !created1 {
		t.Fatalf("first %+v created=%v err=%v", a, created1, err)
	}
	b, created2, err := ops.IssueCredit(ctx, in)
	if err != nil || created2 {
		t.Fatalf("replay created=%v err=%v", created2, err)
	}
	if a.ID != b.ID {
		t.Fatalf("ids %s vs %s", a.ID, b.ID)
	}
}

func TestOverride_rejectedWhenPaid(t *testing.T) {
	store := testStore(t)
	customerID, keyID := seedKey(t, store.Pool)
	ctx := context.Background()
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	_, err := store.InsertBatch(ctx, customerID, keyID, []models.Event{{
		RequestID: "pay-" + keyID,
		Endpoint:  "/x",
		Units:     150000,
		Timestamp: time.Date(2026, 6, 10, 1, 0, 0, 0, time.UTC),
	}})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = (&services.HourService{Windows: store}).Run(ctx, 5000)
	_, err = (&services.InvoiceService{Store: store}).IssuePeriod(ctx, start, end)
	if err != nil {
		t.Fatal(err)
	}

	invID, lineID := firstLine(t, store, customerID, start)
	ops := &services.OpsService{Store: store}
	_, err = ops.ApplyPayment(ctx, models.PaymentEvent{ProviderEventID: "evt-" + invID, InvoiceID: invID})
	if err != nil {
		t.Fatal(err)
	}
	err = ops.OverrideLine(ctx, models.LineOverride{
		InvoiceID: invID, LineID: lineID, AmountMicros: 1, Reason: "nope", Actor: "ops",
	})
	if err != services.ErrInvoicePaid {
		t.Fatalf("got %v", err)
	}
}

func TestApplyPayment_replay(t *testing.T) {
	store := testStore(t)
	customerID, keyID := seedKey(t, store.Pool)
	ctx := context.Background()
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	_, err := store.InsertBatch(ctx, customerID, keyID, []models.Event{{
		RequestID: "wh-" + keyID,
		Endpoint:  "/x",
		Units:     150000,
		Timestamp: time.Date(2026, 7, 10, 1, 0, 0, 0, time.UTC),
	}})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = (&services.HourService{Windows: store}).Run(ctx, 5000)
	_, err = (&services.InvoiceService{Store: store}).IssuePeriod(ctx, start, end)
	if err != nil {
		t.Fatal(err)
	}
	invID, _ := firstLine(t, store, customerID, start)
	ops := &services.OpsService{Store: store}
	ev := models.PaymentEvent{ProviderEventID: "same-" + invID, InvoiceID: invID}
	ok1, err := ops.ApplyPayment(ctx, ev)
	if err != nil || !ok1 {
		t.Fatalf("first applied=%v err=%v", ok1, err)
	}
	ok2, err := ops.ApplyPayment(ctx, ev)
	if err != nil || ok2 {
		t.Fatalf("replay applied=%v err=%v", ok2, err)
	}
}

func firstLine(t *testing.T, store *Store, customerID string, start time.Time) (invoiceID, lineID string) {
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
		SELECT id::text FROM invoices WHERE customer_id = $1 AND period_start = $2
	`, customerID, start).Scan(&invoiceID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT id::text FROM invoice_line_items WHERE invoice_id = $1 ORDER BY position LIMIT 1
	`, invoiceID).Scan(&lineID); err != nil {
		t.Fatal(err)
	}
	return invoiceID, lineID
}
