package controllers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"metered-billing/internal/models"
)

type eventIn struct {
	RequestID string `json:"request_id"`
	Endpoint  string `json:"endpoint"`
	Units     int64  `json:"units"`
	Timestamp string `json:"timestamp"`
}

func (c *Controller) postEvents(w http.ResponseWriter, r *http.Request) {
	key, ok := c.authCustomer(w, r)
	if !ok {
		return
	}

	var body struct {
		Events []eventIn `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	batch := make([]models.Event, 0, len(body.Events))
	for _, e := range body.Events {
		ts, err := time.Parse(time.RFC3339, e.Timestamp)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "timestamp must be RFC3339"})
			return
		}
		batch = append(batch, models.Event{
			RequestID: e.RequestID,
			Endpoint:  e.Endpoint,
			Units:     e.Units,
			Timestamp: ts,
		})
	}

	out, err := c.Ingest.Ingest(r.Context(), key.CustomerID, key.ID, batch)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{
		"inserted":   out.Inserted,
		"duplicates": out.Duplicates,
	})
}

func (c *Controller) health(w http.ResponseWriter, r *http.Request) {
	if c.DB != nil {
		if err := c.DB.Ping(r.Context()); err != nil {
			http.Error(w, "db down", http.StatusServiceUnavailable)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (c *Controller) authCustomer(w http.ResponseWriter, r *http.Request) (models.APIKey, bool) {
	tok, ok := bearer(r.Header.Get("Authorization"))
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return models.APIKey{}, false
	}
	key, err := c.Auth.FromToken(r.Context(), tok)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return models.APIKey{}, false
	}
	return key, true
}

func bearer(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	tok := strings.TrimSpace(header[len(prefix):])
	return tok, tok != ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
