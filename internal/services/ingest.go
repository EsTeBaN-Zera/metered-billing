package services

import (
	"context"
	"fmt"

	"metered-billing/internal/models"
)

type IngestService struct {
	Store EventStore
}

func (s *IngestService) Ingest(ctx context.Context, customerID, apiKeyID string, batch []models.Event) (models.IngestResult, error) {
	if s.Store == nil {
		return models.IngestResult{}, fmt.Errorf("event store is missing")
	}
	if len(batch) == 0 {
		return models.IngestResult{}, fmt.Errorf("empty batch")
	}
	if len(batch) > 500 {
		return models.IngestResult{}, fmt.Errorf("batch too large")
	}
	for _, ev := range batch {
		if err := checkEvent(ev); err != nil {
			return models.IngestResult{}, err
		}
	}
	return s.Store.InsertBatch(ctx, customerID, apiKeyID, batch)
}

func checkEvent(ev models.Event) error {
	if ev.RequestID == "" {
		return fmt.Errorf("request_id is required")
	}
	if len(ev.RequestID) > 128 {
		return fmt.Errorf("request_id too long")
	}
	if ev.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if ev.Units <= 0 {
		return fmt.Errorf("units must be > 0")
	}
	if ev.Timestamp.IsZero() {
		return fmt.Errorf("timestamp is required")
	}
	return nil
}
