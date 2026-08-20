package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"metered-billing/internal/domain"
	"metered-billing/internal/models"
)

func (c *Controller) opsListCustomers(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := c.Ops.ListCustomers(r.Context(), offset, limit)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (c *Controller) opsGetCustomer(w http.ResponseWriter, r *http.Request) {
	d, err := c.Ops.GetCustomer(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, domain.MsgNotFound)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (c *Controller) opsCredit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AmountMicros int64  `json:"amount_micros"`
		Reason       string `json:"reason"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	g, created, err := c.Ops.IssueCredit(r.Context(), models.CreditIssue{
		CustomerID:     r.PathValue("id"),
		AmountMicros:   body.AmountMicros,
		Reason:         body.Reason,
		Actor:          domain.ActorOps,
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{
		"id":               g.ID,
		"remaining_micros": g.RemainingMicros,
		"created":          created,
	})
}

func (c *Controller) opsOverride(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AmountMicros int64  `json:"amount_micros"`
		Reason       string `json:"reason"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	err := c.Ops.OverrideLine(r.Context(), models.LineOverride{
		InvoiceID:    r.PathValue("id"),
		LineID:       r.PathValue("lineId"),
		AmountMicros: body.AmountMicros,
		Reason:       body.Reason,
		Actor:        domain.ActorOps,
	})
	if err != nil {
		if errors.Is(err, domain.ErrInvoicePaid) {
			writeErr(w, http.StatusConflict, domain.MsgInvoicePaid)
			return
		}
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (c *Controller) opsGetInvoice(w http.ResponseWriter, r *http.Request) {
	inv, err := c.Ops.GetInvoice(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, domain.MsgNotFound)
		return
	}
	writeJSON(w, http.StatusOK, inv)
}
