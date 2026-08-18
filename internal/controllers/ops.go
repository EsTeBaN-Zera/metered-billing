package controllers

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"metered-billing/internal/models"
	"metered-billing/internal/services"
)

func (c *Controller) authOps(w http.ResponseWriter, r *http.Request) bool {
	tok, ok := bearer(r.Header.Get("Authorization"))
	if !ok || c.OpsToken == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return false
	}
	if subtle.ConstantTimeCompare([]byte(tok), []byte(c.OpsToken)) != 1 {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return false
	}
	return true
}

func (c *Controller) opsListCustomers(w http.ResponseWriter, r *http.Request) {
	if !c.authOps(w, r) {
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := c.Ops.ListCustomers(r.Context(), offset, limit)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (c *Controller) opsGetCustomer(w http.ResponseWriter, r *http.Request) {
	if !c.authOps(w, r) {
		return
	}
	d, err := c.Ops.GetCustomer(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (c *Controller) opsCredit(w http.ResponseWriter, r *http.Request) {
	if !c.authOps(w, r) {
		return
	}
	var body struct {
		AmountMicros int64  `json:"amount_micros"`
		Reason       string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	g, created, err := c.Ops.IssueCredit(r.Context(), models.CreditIssue{
		CustomerID:     r.PathValue("id"),
		AmountMicros:   body.AmountMicros,
		Reason:         body.Reason,
		Actor:          "ops",
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{
		"id":                g.ID,
		"remaining_micros":  g.RemainingMicros,
		"created":           created,
	})
}

func (c *Controller) opsOverride(w http.ResponseWriter, r *http.Request) {
	if !c.authOps(w, r) {
		return
	}
	var body struct {
		AmountMicros int64  `json:"amount_micros"`
		Reason       string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	err := c.Ops.OverrideLine(r.Context(), models.LineOverride{
		InvoiceID:    r.PathValue("id"),
		LineID:       r.PathValue("lineId"),
		AmountMicros: body.AmountMicros,
		Reason:       body.Reason,
		Actor:        "ops",
	})
	if err != nil {
		if errors.Is(err, services.ErrInvoicePaid) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "invoice is paid"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (c *Controller) opsGetInvoice(w http.ResponseWriter, r *http.Request) {
	if !c.authOps(w, r) {
		return
	}
	inv, err := c.Ops.GetInvoice(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, inv)
}
