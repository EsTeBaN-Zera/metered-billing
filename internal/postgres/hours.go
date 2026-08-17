package postgres

import (
	"context"
	"time"
)

func (s *Store) ProcessDirtyHours(ctx context.Context, limit int) (int, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT set_config('app.is_ops', 'true', true)`); err != nil {
		return 0, err
	}

	rows, err := tx.Query(ctx, `
		SELECT customer_id::text, api_key_id::text, hour_bucket, gen
		FROM dirty_hours
		ORDER BY hour_bucket
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, limit)
	if err != nil {
		return 0, err
	}

	type dirty struct {
		customerID string
		apiKeyID   string
		hour       time.Time
		gen        int64
	}
	var batch []dirty
	for rows.Next() {
		var d dirty
		if err := rows.Scan(&d.customerID, &d.apiKeyID, &d.hour, &d.gen); err != nil {
			rows.Close()
			return 0, err
		}
		batch = append(batch, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	processed := 0
	for _, d := range batch {
		var units int64
		err := tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(units), 0)
			FROM usage_events
			WHERE customer_id = $1
			  AND api_key_id = $2
			  AND event_ts >= $3
			  AND event_ts < $3 + interval '1 hour'
		`, d.customerID, d.apiKeyID, d.hour).Scan(&units)
		if err != nil {
			return 0, err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO usage_windows (customer_id, api_key_id, hour_bucket, units, updated_at)
			VALUES ($1, $2, $3, $4, now())
			ON CONFLICT (customer_id, api_key_id, hour_bucket)
			DO UPDATE SET units = EXCLUDED.units, updated_at = now()
		`, d.customerID, d.apiKeyID, d.hour, units); err != nil {
			return 0, err
		}

		tag, err := tx.Exec(ctx, `
			DELETE FROM dirty_hours
			WHERE customer_id = $1
			  AND api_key_id = $2
			  AND hour_bucket = $3
			  AND gen = $4
		`, d.customerID, d.apiKeyID, d.hour, d.gen)
		if err != nil {
			return 0, err
		}
		if tag.RowsAffected() == 1 {
			processed++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return processed, nil
}
