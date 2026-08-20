package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"metered-billing/internal/domain"
	"metered-billing/internal/models"

	"github.com/jackc/pgx/v5"
)

func (c *Controller) listInvoices(w http.ResponseWriter, r *http.Request) {
	key := apiKeyFrom(r)
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := c.Invoices.List(r.Context(), key.CustomerID, offset, limit)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	items := make([]map[string]any, 0, len(list))
	for _, inv := range list {
		items = append(items, invoiceJSON(inv, false))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (c *Controller) getInvoice(w http.ResponseWriter, r *http.Request) {
	key := apiKeyFrom(r)
	inv, err := c.Invoices.Get(r.Context(), key.CustomerID, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, domain.MsgNotFound)
			return
		}
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, invoiceJSON(inv, true))
}

func invoiceJSON(inv models.Invoice, withLines bool) map[string]any {
	out := map[string]any{
		"id":              inv.ID,
		"period_start":    inv.PeriodStart.UTC().Format(time.RFC3339),
		"period_end":      inv.PeriodEnd.UTC().Format(time.RFC3339),
		"status":          inv.Status,
		"subtotal_micros": inv.SubtotalMicros,
		"credit_micros":   inv.CreditMicros,
		"total_micros":    inv.TotalMicros,
		"issued_at":       inv.IssuedAt.UTC().Format(time.RFC3339),
	}
	if inv.PaidAt != nil {
		out["paid_at"] = inv.PaidAt.UTC().Format(time.RFC3339)
	}
	if withLines {
		lines := make([]map[string]any, 0, len(inv.Lines))
		for _, l := range inv.Lines {
			lines = append(lines, map[string]any{
				"id":            l.ID,
				"kind":          l.Kind,
				"description":   l.Description,
				"quantity":      l.Quantity,
				"unit_micros":   l.UnitMicros,
				"amount_micros": l.AmountMicros,
			})
		}
		out["line_items"] = lines
	}
	return out
}
