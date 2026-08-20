package services

import (
	"context"

	"metered-billing/internal/domain"
	"metered-billing/internal/models"
)

type IngestService struct {
	Store domain.EventStore
}

func (s *IngestService) Ingest(ctx context.Context, customerID, apiKeyID string, batch []models.Event) (models.IngestResult, error) {
	if s.Store == nil {
		return models.IngestResult{}, domain.ErrEventStoreMissing
	}
	if len(batch) == 0 {
		return models.IngestResult{}, domain.ErrEmptyBatch
	}
	if len(batch) > domain.MaxEventBatch {
		return models.IngestResult{}, domain.ErrBatchTooLarge
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
		return domain.ErrRequestIDRequired
	}
	if len(ev.RequestID) > domain.MaxRequestIDLen {
		return domain.ErrRequestIDTooLong
	}
	if ev.Endpoint == "" {
		return domain.ErrEndpointRequired
	}
	if ev.Units <= 0 {
		return domain.ErrUnitsMustBePositive
	}
	if ev.Timestamp.IsZero() {
		return domain.ErrTimestampRequired
	}
	return nil
}
