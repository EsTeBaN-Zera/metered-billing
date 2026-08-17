package postgres

import (
	"context"
	"fmt"

	"metered-billing/internal/models"

	"github.com/jackc/pgx/v5"
)

func (s *Store) ListWindows(ctx context.Context, customerID string, q models.UsageQuery) ([]models.UsageWindow, error) {
	var out []models.UsageWindow
	err := s.withCustomer(ctx, customerID, func(tx pgx.Tx) error {
		sql := `
			SELECT customer_id::text, api_key_id::text, hour_bucket, units
			FROM usage_windows
			WHERE hour_bucket >= $1 AND hour_bucket < $2
		`
		args := []any{q.From, q.To}
		n := 3
		if q.APIKeyID != "" {
			sql += fmt.Sprintf(" AND api_key_id = $%d", n)
			args = append(args, q.APIKeyID)
			n++
		}
		if !q.AfterHour.IsZero() {
			sql += fmt.Sprintf(" AND (hour_bucket, api_key_id::text) > ($%d, $%d)", n, n+1)
			args = append(args, q.AfterHour, q.AfterKey)
			n += 2
		}
		sql += fmt.Sprintf(" ORDER BY hour_bucket, api_key_id LIMIT $%d", n)
		args = append(args, q.Limit)

		rows, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var w models.UsageWindow
			if err := rows.Scan(&w.CustomerID, &w.APIKeyID, &w.Hour, &w.Units); err != nil {
				return err
			}
			out = append(out, w)
		}
		return rows.Err()
	})
	return out, err
}
