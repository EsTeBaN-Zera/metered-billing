package postgres

import (
	"context"
	"encoding/json"

	"metered-billing/internal/domain"
	"metered-billing/internal/models"

	"github.com/jackc/pgx/v5"
)

func (s *Store) ListCustomers(ctx context.Context, offset, limit int) ([]models.Customer, error) {
	var out []models.Customer
	err := s.withOps(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, name, created_at
			FROM customers
			ORDER BY name
			OFFSET $1 LIMIT $2
		`, offset, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c models.Customer
			if err := rows.Scan(&c.ID, &c.Name, &c.CreatedAt); err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Store) GetCustomer(ctx context.Context, id string) (models.CustomerDetail, error) {
	var d models.CustomerDetail
	err := s.withOps(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT id::text, name, created_at FROM customers WHERE id = $1
		`, id).Scan(&d.ID, &d.Name, &d.CreatedAt)
		if err != nil {
			return err
		}

		_ = tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(units), 0)
			FROM usage_windows
			WHERE customer_id = $1
			  AND hour_bucket >= date_trunc('day', now())
			  AND hour_bucket < date_trunc('day', now()) + interval '1 day'
		`, id).Scan(&d.TodayUnits)

		_ = tx.QueryRow(ctx, `
			SELECT COALESCE(AVG(day_units), 0)
			FROM (
			  SELECT SUM(units) AS day_units
			  FROM usage_windows
			  WHERE customer_id = $1
			    AND hour_bucket >= now() - make_interval(days => $2)
			  GROUP BY date_trunc('day', hour_bucket)
			) t
		`, id, domain.AvgWindowDays).Scan(&d.Avg30Units)
		d.Anomaly = d.Avg30Units > 0 && float64(d.TodayUnits) > float64(domain.AnomalyFactor)*d.Avg30Units

		rows, err := tx.Query(ctx, `
			SELECT id::text, customer_id::text, period_start, period_end, status,
			       subtotal_micros, credit_applied_micros, total_micros, issued_at, paid_at
			FROM invoices WHERE customer_id = $1
			ORDER BY issued_at DESC LIMIT $2
		`, id, domain.OpsInvoiceListLimit)
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
			d.Invoices = append(d.Invoices, inv)
		}
		return rows.Err()
	})
	return d, err
}

func (s *Store) IssueCredit(ctx context.Context, in models.CreditIssue) (models.CreditGrant, bool, error) {
	var g models.CreditGrant
	created := false
	err := s.withOps(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO credit_grants (
				customer_id, amount_micros, remaining_micros, reason, actor, idempotency_key
			) VALUES ($1, $2, $2, $3, $4, $5)
			ON CONFLICT (idempotency_key) DO NOTHING
			RETURNING id::text, remaining_micros
		`, in.CustomerID, in.AmountMicros, in.Reason, in.Actor, in.IdempotencyKey).Scan(&g.ID, &g.RemainingMicros)
		if err == pgx.ErrNoRows {
			return tx.QueryRow(ctx, `
				SELECT id::text, remaining_micros FROM credit_grants WHERE idempotency_key = $1
			`, in.IdempotencyKey).Scan(&g.ID, &g.RemainingMicros)
		}
		if err != nil {
			return err
		}
		after, _ := json.Marshal(map[string]any{
			"amount_micros": in.AmountMicros,
			"customer_id":   in.CustomerID,
		})
		if _, err = tx.Exec(ctx, `
			INSERT INTO audit_entries (actor, action, entity_type, entity_id, before_json, after_json, reason)
			VALUES ($1, 'credit_issued', 'credit_grant', $2, '{}', $3, $4)
		`, in.Actor, g.ID, after, in.Reason); err != nil {
			return err
		}
		created = true
		return nil
	})
	return g, created, err
}

func (s *Store) OverrideLine(ctx context.Context, in models.LineOverride) error {
	return s.withOps(ctx, func(tx pgx.Tx) error {
		var invoiceID, status string
		var oldAmount int64
		err := tx.QueryRow(ctx, `
			SELECT i.id::text, i.status, li.amount_micros
			FROM invoice_line_items li
			JOIN invoices i ON i.id = li.invoice_id
			WHERE li.id = $1 AND i.id = $2
			FOR UPDATE OF i
		`, in.LineID, in.InvoiceID).Scan(&invoiceID, &status, &oldAmount)
		if err != nil {
			return err
		}
		if status == domain.StatusPaid {
			return domain.ErrInvoicePaid
		}

		if _, err := tx.Exec(ctx, `
			UPDATE invoice_line_items SET amount_micros = $2 WHERE id = $1
		`, in.LineID, in.AmountMicros); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			UPDATE invoices i SET
			  subtotal_micros = GREATEST(0, x.sum_pos),
			  credit_applied_micros = GREATEST(0, -x.sum_neg),
			  total_micros = GREATEST(0, x.sum_all)
			FROM (
			  SELECT
			    COALESCE(SUM(CASE WHEN amount_micros > 0 THEN amount_micros ELSE 0 END), 0) AS sum_pos,
			    COALESCE(SUM(CASE WHEN amount_micros < 0 THEN amount_micros ELSE 0 END), 0) AS sum_neg,
			    COALESCE(SUM(amount_micros), 0) AS sum_all
			  FROM invoice_line_items WHERE invoice_id = $1
			) x
			WHERE i.id = $1
		`, in.InvoiceID); err != nil {
			return err
		}

		before, _ := json.Marshal(map[string]int64{"amount_micros": oldAmount})
		after, _ := json.Marshal(map[string]int64{"amount_micros": in.AmountMicros})
		_, err = tx.Exec(ctx, `
			INSERT INTO audit_entries (actor, action, entity_type, entity_id, before_json, after_json, reason)
			VALUES ($1, 'line_item_overridden', 'invoice_line_item', $2, $3, $4, $5)
		`, in.Actor, in.LineID, before, after, in.Reason)
		return err
	})
}

func (s *Store) ApplyPayment(ctx context.Context, ev models.PaymentEvent) (bool, error) {
	applied := false
	err := s.withOps(ctx, func(tx pgx.Tx) error {
		var dummy string
		err := tx.QueryRow(ctx, `
			INSERT INTO webhook_events (provider_event_id, invoice_id)
			VALUES ($1, $2)
			ON CONFLICT (provider_event_id) DO NOTHING
			RETURNING provider_event_id
		`, ev.ProviderEventID, ev.InvoiceID).Scan(&dummy)
		if err == pgx.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			UPDATE invoices SET status = $2, paid_at = now()
			WHERE id = $1 AND status = $3
		`, ev.InvoiceID, domain.StatusPaid, domain.StatusIssued)
		if err != nil {
			return err
		}
		applied = tag.RowsAffected() == 1
		return nil
	})
	return applied, err
}
