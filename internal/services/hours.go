package services

import (
	"context"
	"fmt"
)

type HourService struct {
	Windows WindowStore
}

func (s *HourService) Run(ctx context.Context, limit int) (int, error) {
	if s.Windows == nil {
		return 0, fmt.Errorf("window store is missing")
	}
	if limit <= 0 {
		limit = 100
	}
	return s.Windows.ProcessDirtyHours(ctx, limit)
}
