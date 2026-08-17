package postgres

import (
	"context"
	"time"

	"metered-billing/internal/models"
)

func (s *Store) InsertBatch(ctx context.Context, customerID, apiKeyID string, batch []models.Event) (models.IngestResult, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return models.IngestResult{}, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT set_config('app.customer_id', $1, true)`, customerID); err != nil {
		return models.IngestResult{}, err
	}

	var out models.IngestResult
	for _, ev := range batch {
		hour := ev.Timestamp.UTC().Truncate(time.Hour)
		tag, err := tx.Exec(ctx, `
			INSERT INTO usage_events (request_id, customer_id, api_key_id, endpoint, units, event_ts)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (request_id) DO NOTHING
		`, ev.RequestID, customerID, apiKeyID, ev.Endpoint, ev.Units, ev.Timestamp.UTC())
		if err != nil {
			return models.IngestResult{}, err
		}
		if tag.RowsAffected() == 0 {
			out.Duplicates++
			continue
		}
		out.Inserted++
		if _, err := tx.Exec(ctx, `
			INSERT INTO dirty_hours (customer_id, api_key_id, hour_bucket, gen)
			VALUES ($1, $2, $3, 1)
			ON CONFLICT (customer_id, api_key_id, hour_bucket)
			DO UPDATE SET gen = dirty_hours.gen + 1
		`, customerID, apiKeyID, hour); err != nil {
			return models.IngestResult{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return models.IngestResult{}, err
	}
	return out, nil
}
