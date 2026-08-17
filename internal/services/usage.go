package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"metered-billing/internal/models"
)

type UsageService struct {
	Store UsageStore
}

func (s *UsageService) List(ctx context.Context, customerID string, q models.UsageQuery) (models.UsagePage, error) {
	if s.Store == nil {
		return models.UsagePage{}, fmt.Errorf("usage store is missing")
	}
	if q.To.IsZero() || q.From.IsZero() {
		return models.UsagePage{}, fmt.Errorf("from and to are required")
	}
	if !q.To.After(q.From) {
		return models.UsagePage{}, fmt.Errorf("to must be after from")
	}
	if q.Limit <= 0 {
		q.Limit = 50
	}
	if q.Limit > 200 {
		q.Limit = 200
	}
	if q.Cursor != "" {
		hour, key, err := DecodeCursor(q.Cursor)
		if err != nil {
			return models.UsagePage{}, err
		}
		q.AfterHour = hour
		q.AfterKey = key
	}
	q.Limit++
	rows, err := s.Store.ListWindows(ctx, customerID, q)
	if err != nil {
		return models.UsagePage{}, err
	}
	page := models.UsagePage{Windows: rows}
	if len(rows) == q.Limit {
		last := rows[len(rows)-2]
		page.Windows = rows[:len(rows)-1]
		page.NextCursor = EncodeCursor(last.Hour, last.APIKeyID)
	}
	return page, nil
}

func EncodeCursor(hour time.Time, apiKeyID string) string {
	raw := hour.UTC().Format(time.RFC3339) + "|" + apiKeyID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func DecodeCursor(s string) (time.Time, string, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("bad cursor")
	}
	hourStr, key, ok := strings.Cut(string(b), "|")
	if !ok {
		return time.Time{}, "", fmt.Errorf("bad cursor")
	}
	hour, err := time.Parse(time.RFC3339, hourStr)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("bad cursor")
	}
	return hour, key, nil
}
