package controllers

import (
	"net/http"
	"strconv"
	"time"

	"metered-billing/internal/domain"
	"metered-billing/internal/models"
)

func (c *Controller) getUsage(w http.ResponseWriter, r *http.Request) {
	key := apiKeyFrom(r)
	q := r.URL.Query()
	from, err1 := time.Parse(time.RFC3339, q.Get("from"))
	to, err2 := time.Parse(time.RFC3339, q.Get("to"))
	if err1 != nil || err2 != nil {
		writeErr(w, http.StatusBadRequest, domain.MsgFromToRFC3339)
		return
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	page, err := c.Usage.List(r.Context(), key.CustomerID, models.UsageQuery{
		From:     from,
		To:       to,
		APIKeyID: q.Get("api_key"),
		Cursor:   q.Get("cursor"),
		Limit:    limit,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	type row struct {
		APIKeyID string `json:"api_key_id"`
		Hour     string `json:"hour"`
		Units    int64  `json:"units"`
	}
	items := make([]row, 0, len(page.Windows))
	for _, wdw := range page.Windows {
		items = append(items, row{
			APIKeyID: wdw.APIKeyID,
			Hour:     wdw.Hour.UTC().Format(time.RFC3339),
			Units:    wdw.Units,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"next_cursor": page.NextCursor,
	})
}
