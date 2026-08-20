package services

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"metered-billing/internal/domain"
	"metered-billing/internal/models"
)

type UsageService struct {
	Store domain.UsageStore
}

func (s *UsageService) List(ctx context.Context, customerID string, q models.UsageQuery) (models.UsagePage, error) {
	if s.Store == nil {
		return models.UsagePage{}, domain.ErrUsageStoreMissing
	}
	if q.To.IsZero() || q.From.IsZero() {
		return models.UsagePage{}, domain.ErrFromAndToRequired
	}
	if !q.To.After(q.From) {
		return models.UsagePage{}, domain.ErrToAfterFrom
	}
	if q.Limit <= 0 {
		q.Limit = domain.DefaultPageSize
	}
	if q.Limit > domain.MaxUsagePage {
		q.Limit = domain.MaxUsagePage
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
		return time.Time{}, "", domain.ErrBadCursor
	}
	hourStr, key, ok := strings.Cut(string(b), "|")
	if !ok {
		return time.Time{}, "", domain.ErrBadCursor
	}
	hour, err := time.Parse(time.RFC3339, hourStr)
	if err != nil {
		return time.Time{}, "", domain.ErrBadCursor
	}
	return hour, key, nil
}
