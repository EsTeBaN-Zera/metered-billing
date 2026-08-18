package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"metered-billing/internal/models"
	"metered-billing/internal/postgres"
	"metered-billing/internal/services"

	"math/rand/v2"
)

const planID = "11111111-1111-1111-1111-111111111111"

func main() {
	ctx := context.Background()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	pepper := os.Getenv("KEY_PEPPER")
	if pepper == "" {
		log.Fatal("KEY_PEPPER is required")
	}

	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	if seeded(ctx, store) {
		log.Print("seed already ran, skip")
		return
	}

	hash := services.PepperHash{Pepper: pepper}
	ingest := &services.IngestService{Store: store}
	hours := &services.HourService{Windows: store}
	invoices := &services.InvoiceService{Store: store, Clock: services.RealClock{}}

	now := time.Now().UTC()
	prevStart, prevEnd := services.PreviousMonth(now)
	thisStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	type spec struct {
		name  string
		nKeys int
		busy  bool
		spike bool
	}
	specs := []spec{
		{"seed:Acme", 2, true, false},
		{"seed:Globex", 1, true, false},
		{"seed:Initech", 1, false, false},
		{"seed:Spike", 1, true, true},
	}

	fmt.Println("API keys (plaintext once):")
	for _, s := range specs {
		cid, keys := createFirm(ctx, store, hash, s.name, s.nKeys)
		for i, k := range keys {
			fmt.Printf("  %s key%d  %s\n", s.name, i+1, k.plain)
			evs := eventsFor(k.id, prevStart, prevEnd, s.busy)
			evs = append(evs, eventsFor(k.id, thisStart, now, s.busy)...)
			if s.busy {
				evs = append(evs, models.Event{
					RequestID: k.id + "-july-floor",
					Endpoint:  "/v1/translate",
					Timestamp: prevStart.Add(36 * time.Hour),
					Units:     25000,
				})
			}
			if s.spike {
				evs = append(evs, spikeToday(k.id, now)...)
			}
			flush(ctx, ingest, cid, k.id, evs)
		}
	}

	for i := 0; i < 200; i++ {
		n, err := hours.Run(ctx, 500)
		if err != nil {
			log.Fatal(err)
		}
		if n == 0 {
			break
		}
		if i == 199 {
			log.Fatal("dirty hours still left after seed")
		}
	}

	issued, err := invoices.IssuePeriod(ctx, prevStart, prevEnd)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("seed done, invoices issued for previous month: %d", issued)
}

type apiKey struct {
	id    string
	plain string
}

func seeded(ctx context.Context, store *postgres.Store) bool {
	tx, err := store.Pool.Begin(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.is_ops', 'true', true)`); err != nil {
		log.Fatal(err)
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM customers WHERE name LIKE 'seed:%'`).Scan(&n); err != nil {
		log.Fatal(err)
	}
	return n > 0
}

func createFirm(ctx context.Context, store *postgres.Store, hash services.PepperHash, name string, nKeys int) (string, []apiKey) {
	tx, err := store.Pool.Begin(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.is_ops', 'true', true)`); err != nil {
		log.Fatal(err)
	}
	var cid string
	if err := tx.QueryRow(ctx, `
		INSERT INTO customers (name, price_plan_id) VALUES ($1, $2) RETURNING id::text
	`, name, planID).Scan(&cid); err != nil {
		log.Fatal(err)
	}
	keys := make([]apiKey, 0, nKeys)
	for i := 0; i < nKeys; i++ {
		plain, prefix, err := services.NewPlaintext()
		if err != nil {
			log.Fatal(err)
		}
		var kid string
		if err := tx.QueryRow(ctx, `
			INSERT INTO api_keys (customer_id, name, prefix, key_hash)
			VALUES ($1, $2, $3, $4) RETURNING id::text
		`, cid, fmt.Sprintf("key-%d", i+1), prefix, hash.Sum(plain)).Scan(&kid); err != nil {
			log.Fatal(err)
		}
		keys = append(keys, apiKey{id: kid, plain: plain})
	}
	if err := tx.Commit(ctx); err != nil {
		log.Fatal(err)
	}
	return cid, keys
}

func eventsFor(keyID string, from, to time.Time, busy bool) []models.Event {
	var out []models.Event
	for ts := from; ts.Before(to); ts = ts.Add(time.Hour) {
		h := ts.UTC().Hour()
		if busy && (h < 8 || h > 19) && rand.IntN(100) < 70 {
			continue
		}
		if !busy && rand.IntN(100) < 50 {
			continue
		}
		units := int64(5 + rand.IntN(40))
		if busy && h >= 9 && h <= 17 {
			units += int64(rand.IntN(80))
		}
		out = append(out, models.Event{
			RequestID: fmt.Sprintf("%s-%d", keyID, ts.Unix()),
			Endpoint:  pickEndpoint(),
			Timestamp: ts.Add(time.Duration(rand.IntN(50)) * time.Minute),
			Units:     units,
		})
	}
	return out
}

func spikeToday(keyID string, now time.Time) []models.Event {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	var out []models.Event
	for i := 0; i < 24; i++ {
		ts := start.Add(time.Duration(i) * time.Hour)
		if ts.After(now) {
			break
		}
		out = append(out, models.Event{
			RequestID: fmt.Sprintf("%s-spike-%d", keyID, i),
			Endpoint:  "/v1/translate",
			Timestamp: ts.Add(10 * time.Minute),
			Units:     8000,
		})
	}
	return out
}

func pickEndpoint() string {
	eps := []string{"/v1/translate", "/v1/search", "/v1/embed"}
	return eps[rand.IntN(len(eps))]
}

func flush(ctx context.Context, ingest *services.IngestService, customerID, keyID string, evs []models.Event) {
	for i := 0; i < len(evs); i += 200 {
		end := min(i+200, len(evs))
		if _, err := ingest.Ingest(ctx, customerID, keyID, evs[i:end]); err != nil {
			log.Fatal(err)
		}
	}
}
