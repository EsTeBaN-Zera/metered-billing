package controllers

import (
	"encoding/json"
	"net/http"
	"time"

	"metered-billing/internal/domain"
	"metered-billing/internal/models"
)

type eventIn struct {
	RequestID string `json:"request_id"`
	Endpoint  string `json:"endpoint"`
	Units     int64  `json:"units"`
	Timestamp string `json:"timestamp"`
}

func (c *Controller) postEvents(w http.ResponseWriter, r *http.Request) {
	key := apiKeyFrom(r)

	var body struct {
		Events []eventIn `json:"events"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	batch := make([]models.Event, 0, len(body.Events))
	for _, e := range body.Events {
		ts, err := time.Parse(time.RFC3339, e.Timestamp)
		if err != nil {
			writeErr(w, http.StatusBadRequest, domain.MsgTimestampRFC3339)
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
		writeErr(w, http.StatusBadRequest, err.Error())
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
			http.Error(w, domain.MsgDBDown, http.StatusServiceUnavailable)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", domain.JSONContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
