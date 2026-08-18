package controllers

import (
	"encoding/json"
	"io"
	"net/http"

	"metered-billing/internal/models"
)

func (c *Controller) webhookPayment(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	sig := r.Header.Get("X-Webhook-Signature")
	if c.WebhookCheck == nil || !c.WebhookCheck.Valid(body, sig) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var ev struct {
		EventID   string `json:"event_id"`
		InvoiceID string `json:"invoice_id"`
	}
	if err := json.Unmarshal(body, &ev); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	_, err = c.Ops.ApplyPayment(r.Context(), models.PaymentEvent{
		ProviderEventID: ev.EventID,
		InvoiceID:       ev.InvoiceID,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
