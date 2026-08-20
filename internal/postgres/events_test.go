package postgres

import (
	"context"
	"sync"
	"testing"
	"time"

	"metered-billing/internal/domain"
	"metered-billing/internal/models"
	"metered-billing/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	url := domain.TestDatabaseURL
	store, err := Connect(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	return store
}

func seedKey(t *testing.T, pool *pgxpool.Pool) (customerID, keyID string) {
	t.Helper()
	ctx := context.Background()
	pepper := services.PepperHash{Pepper: domain.TestKeyPepper}
	plaintext, prefix, err := services.NewPlaintext()
	if err != nil {
		t.Fatal(err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT set_config('app.is_ops', 'true', true)`); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO customers (name, price_plan_id)
		VALUES ($1, $2)
		RETURNING id::text
	`, "test-"+prefix, domain.StandardPlanID).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO api_keys (customer_id, name, prefix, key_hash)
		VALUES ($1, 'test', $2, $3)
		RETURNING id::text
	`, customerID, prefix, pepper.Sum(plaintext)).Scan(&keyID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return customerID, keyID
}

func TestInsertBatch_sameRequestIDDoesNotDouble(t *testing.T) {
	store := testStore(t)
	customerID, keyID := seedKey(t, store.Pool)
	ctx := context.Background()

	ev := models.Event{
		RequestID: "req-" + keyID,
		Endpoint:  "/translate",
		Units:     3,
		Timestamp: time.Date(2026, 3, 15, 14, 3, 0, 0, time.UTC),
	}

	first, err := store.InsertBatch(ctx, customerID, keyID, []models.Event{ev})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.InsertBatch(ctx, customerID, keyID, []models.Event{ev})
	if err != nil {
		t.Fatal(err)
	}
	if first.Inserted != 1 || second.Duplicates != 1 {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	if n := countEvents(t, store.Pool, ev.RequestID); n != 1 {
		t.Fatalf("got %d rows", n)
	}
}

func TestInsertBatch_concurrentSameRequestID(t *testing.T) {
	store := testStore(t)
	customerID, keyID := seedKey(t, store.Pool)
	ctx := context.Background()

	ev := models.Event{
		RequestID: "req-concurrent-" + keyID,
		Endpoint:  "/search",
		Units:     5,
		Timestamp: time.Date(2026, 3, 15, 14, 40, 0, 0, time.UTC),
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.InsertBatch(ctx, customerID, keyID, []models.Event{ev})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := countEvents(t, store.Pool, ev.RequestID); n != 1 {
		t.Fatalf("concurrent insert stored %d rows", n)
	}
}

func countEvents(t *testing.T, pool *pgxpool.Pool, requestID string) int {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.is_ops', 'true', true)`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM usage_events WHERE request_id = $1`, requestID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
