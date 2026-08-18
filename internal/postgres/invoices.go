package postgres

import (
	"context"
	"time"

	"metered-billing/internal/models"

	"github.com/jackc/pgx/v5"
)

func (s *Store) withOps(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.is_ops', 'true', true)`); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) withCustomer(ctx context.Context, customerID string, fn func(tx pgx.Tx) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.customer_id', $1, true)`, customerID); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) CustomersWithUsage(ctx context.Context, start, end time.Time) ([]string, error) {
	var ids []string
	err := s.withOps(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT DISTINCT w.customer_id::text
			FROM usage_windows w
			WHERE w.hour_bucket >= $1 AND w.hour_bucket < $2
			  AND NOT EXISTS (
			    SELECT 1 FROM invoices i
			    WHERE i.customer_id = w.customer_id AND i.period_start = $1
			  )
			  AND NOT EXISTS (
			    SELECT 1 FROM dirty_hours d
			    WHERE d.customer_id = w.customer_id
			      AND d.hour_bucket >= $1 AND d.hour_bucket < $2
			  )
		`, start, end)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	return ids, err
}

func (s *Store) SumUnits(ctx context.Context, customerID string, start, end time.Time) (int64, error) {
	var units int64
	err := s.withOps(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(units), 0)
			FROM usage_windows
			WHERE customer_id = $1 AND hour_bucket >= $2 AND hour_bucket < $3
		`, customerID, start, end).Scan(&units)
	})
	return units, err
}

func (s *Store) Tiers(ctx context.Context, customerID string) ([]models.Tier, error) {
	var tiers []models.Tier
	err := s.withOps(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT t.from_units, t.to_units, t.unit_price_micros, t.sort_order
			FROM price_plan_tiers t
			JOIN customers c ON c.price_plan_id = t.plan_id
			WHERE c.id = $1
			ORDER BY t.sort_order
		`, customerID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var t models.Tier
			if err := rows.Scan(&t.From, &t.To, &t.PriceMicros, &t.SortOrder); err != nil {
				return err
			}
			tiers = append(tiers, t)
		}
		return rows.Err()
	})
	return tiers, err
}

func (s *Store) RemainingCredits(ctx context.Context, customerID string) ([]models.CreditGrant, error) {
	var grants []models.CreditGrant
	err := s.withOps(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, remaining_micros
			FROM credit_grants
			WHERE customer_id = $1 AND remaining_micros > 0
			ORDER BY created_at
		`, customerID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var g models.CreditGrant
			if err := rows.Scan(&g.ID, &g.RemainingMicros); err != nil {
				return err
			}
			grants = append(grants, g)
		}
		return rows.Err()
	})
	return grants, err
}

func (s *Store) SaveInvoice(ctx context.Context, inv models.NewInvoice) (bool, error) {
	created := false
	err := s.withOps(ctx, func(tx pgx.Tx) error {
		var id string
		err := tx.QueryRow(ctx, `
			INSERT INTO invoices (
				customer_id, period_start, period_end, status,
				subtotal_micros, credit_applied_micros, total_micros
			) VALUES ($1, $2, $3, 'issued', $4, $5, $6)
			ON CONFLICT (customer_id, period_start) DO NOTHING
			RETURNING id::text
		`, inv.CustomerID, inv.PeriodStart, inv.PeriodEnd,
			inv.SubtotalMicros, inv.CreditMicros, inv.TotalMicros,
		).Scan(&id)
		if err == pgx.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		for _, line := range inv.Lines {
			if _, err := tx.Exec(ctx, `
				INSERT INTO invoice_line_items (
					invoice_id, kind, description, quantity_units,
					unit_price_micros, amount_micros, position
				) VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, id, line.Kind, line.Description, line.Quantity,
				line.UnitMicros, line.AmountMicros, line.Position); err != nil {
				return err
			}
		}
		for _, g := range inv.Credits {
			if _, err := tx.Exec(ctx, `
				UPDATE credit_grants SET remaining_micros = $2 WHERE id = $1
			`, g.ID, g.RemainingMicros); err != nil {
				return err
			}
		}
		created = true
		return nil
	})
	return created, err
}

func (s *Store) ListInvoices(ctx context.Context, customerID string, offset, limit int) ([]models.Invoice, error) {
	var out []models.Invoice
	err := s.withCustomer(ctx, customerID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, customer_id::text, period_start, period_end, status,
			       subtotal_micros, credit_applied_micros, total_micros, issued_at, paid_at
			FROM invoices
			ORDER BY issued_at DESC
			OFFSET $1 LIMIT $2
		`, offset, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var inv models.Invoice
			if err := rows.Scan(
				&inv.ID, &inv.CustomerID, &inv.PeriodStart, &inv.PeriodEnd, &inv.Status,
				&inv.SubtotalMicros, &inv.CreditMicros, &inv.TotalMicros, &inv.IssuedAt, &inv.PaidAt,
			); err != nil {
				return err
			}
			out = append(out, inv)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Store) GetInvoice(ctx context.Context, customerID, invoiceID string) (models.Invoice, error) {
	var inv models.Invoice
	err := s.withCustomer(ctx, customerID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT id::text, customer_id::text, period_start, period_end, status,
			       subtotal_micros, credit_applied_micros, total_micros, issued_at, paid_at
			FROM invoices WHERE id = $1
		`, invoiceID).Scan(
			&inv.ID, &inv.CustomerID, &inv.PeriodStart, &inv.PeriodEnd, &inv.Status,
			&inv.SubtotalMicros, &inv.CreditMicros, &inv.TotalMicros, &inv.IssuedAt, &inv.PaidAt,
		)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT id::text, kind, description, quantity_units, unit_price_micros, amount_micros, position
			FROM invoice_line_items WHERE invoice_id = $1 ORDER BY position
		`, invoiceID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var line models.LineItem
			if err := rows.Scan(&line.ID, &line.Kind, &line.Description, &line.Quantity, &line.UnitMicros, &line.AmountMicros, &line.Position); err != nil {
				return err
			}
			inv.Lines = append(inv.Lines, line)
		}
		return rows.Err()
	})
	return inv, err
}

func (s *Store) GetInvoiceByID(ctx context.Context, invoiceID string) (models.Invoice, error) {
	var inv models.Invoice
	err := s.withOps(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT id::text, customer_id::text, period_start, period_end, status,
			       subtotal_micros, credit_applied_micros, total_micros, issued_at, paid_at
			FROM invoices WHERE id = $1
		`, invoiceID).Scan(
			&inv.ID, &inv.CustomerID, &inv.PeriodStart, &inv.PeriodEnd, &inv.Status,
			&inv.SubtotalMicros, &inv.CreditMicros, &inv.TotalMicros, &inv.IssuedAt, &inv.PaidAt,
		)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT id::text, kind, description, quantity_units, unit_price_micros, amount_micros, position
			FROM invoice_line_items WHERE invoice_id = $1 ORDER BY position
		`, invoiceID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var line models.LineItem
			if err := rows.Scan(&line.ID, &line.Kind, &line.Description, &line.Quantity, &line.UnitMicros, &line.AmountMicros, &line.Position); err != nil {
				return err
			}
			inv.Lines = append(inv.Lines, line)
		}
		return rows.Err()
	})
	return inv, err
}
