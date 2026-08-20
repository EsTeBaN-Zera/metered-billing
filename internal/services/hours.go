package services

import (
	"context"

	"metered-billing/internal/domain"
)

type HourService struct {
	Windows domain.WindowStore
}

func (s *HourService) Run(ctx context.Context, limit int) (int, error) {
	if s.Windows == nil {
		return 0, domain.ErrWindowStoreMissing
	}
	if limit <= 0 {
		limit = domain.DefaultHourJobLimit
	}
	return s.Windows.ProcessDirtyHours(ctx, limit)
}
