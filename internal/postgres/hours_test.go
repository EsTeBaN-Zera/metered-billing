package postgres

import (
	"context"
	"testing"
	"time"

	"metered-billing/internal/models"
	"metered-billing/internal/services"
)

func TestProcessDirtyHours_onlyNewRows(t *testing.T) {
	store := testStore(t)
	customerID, keyID := seedKey(t, store.Pool)
	ctx := context.Background()
	hour := time.Date(2026, 3, 15, 14, 0, 0, 0, time.UTC)

	_, err := store.InsertBatch(ctx, customerID, keyID, []models.Event{
		{RequestID: "h1-" + keyID, Endpoint: "/a", Units: 10, Timestamp: hour.Add(3 * time.Minute)},
		{RequestID: "h2-" + keyID, Endpoint: "/a", Units: 5, Timestamp: hour.Add(40 * time.Minute)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := dirtyCount(t, store, customerID, keyID, hour); n != 1 {
		t.Fatalf("dirty after ingest=%d want 1", n)
	}

	hours := &services.HourService{Windows: store}
	if _, err := hours.Run(ctx, 500); err != nil {
		t.Fatal(err)
	}
	if n := dirtyCount(t, store, customerID, keyID, hour); n != 0 {
		t.Fatalf("dirty after job=%d want 0", n)
	}
	if u := windowUnits(t, store, customerID, keyID, hour); u != 15 {
		t.Fatalf("window units=%d want 15", u)
	}

	if _, err := hours.Run(ctx, 500); err != nil {
		t.Fatal(err)
	}
	if n := dirtyCount(t, store, customerID, keyID, hour); n != 0 {
		t.Fatalf("dirty after second job=%d want 0", n)
	}

	_, err = store.InsertBatch(ctx, customerID, keyID, []models.Event{
		{RequestID: "h3-" + keyID, Endpoint: "/a", Units: 2, Timestamp: hour.Add(50 * time.Minute)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := dirtyCount(t, store, customerID, keyID, hour); n != 1 {
		t.Fatalf("dirty after new event=%d want 1", n)
	}
	if _, err := hours.Run(ctx, 500); err != nil {
		t.Fatal(err)
	}
	if n := dirtyCount(t, store, customerID, keyID, hour); n != 0 {
		t.Fatalf("dirty after recompute=%d want 0", n)
	}
	if u := windowUnits(t, store, customerID, keyID, hour); u != 17 {
		t.Fatalf("window units=%d want 17", u)
	}
}

func windowUnits(t *testing.T, store *Store, customerID, keyID string, hour time.Time) int64 {
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
	var u int64
	err = tx.QueryRow(ctx, `
		SELECT units FROM usage_windows
		WHERE customer_id = $1 AND api_key_id = $2 AND hour_bucket = $3
	`, customerID, keyID, hour).Scan(&u)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func dirtyCount(t *testing.T, store *Store, customerID, keyID string, hour time.Time) int {
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
		SELECT count(*) FROM dirty_hours
		WHERE customer_id = $1 AND api_key_id = $2 AND hour_bucket = $3
	`, customerID, keyID, hour).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}
