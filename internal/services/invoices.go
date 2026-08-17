package services

import (
	"context"
	"fmt"
	"time"

	"metered-billing/internal/models"
)

type InvoiceService struct {
	Store InvoiceStore
	Clock Clock
}

func PreviousMonth(now time.Time) (start, end time.Time) {
	now = now.UTC()
	end = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	start = end.AddDate(0, -1, 0)
	return start, end
}

func (s *InvoiceService) IssuePreviousMonth(ctx context.Context) (int, error) {
	if s.Clock == nil {
		s.Clock = RealClock{}
	}
	start, end := PreviousMonth(s.Clock.Now())
	return s.IssuePeriod(ctx, start, end)
}

func (s *InvoiceService) IssuePeriod(ctx context.Context, start, end time.Time) (int, error) {
	if s.Store == nil {
		return 0, fmt.Errorf("invoice store is missing")
	}
	ids, err := s.Store.CustomersWithUsage(ctx, start, end)
	if err != nil {
		return 0, err
	}
	issued := 0
	for _, id := range ids {
		ok, err := s.issueOne(ctx, id, start, end)
		if err != nil {
			return issued, err
		}
		if ok {
			issued++
		}
	}
	return issued, nil
}

func (s *InvoiceService) issueOne(ctx context.Context, customerID string, start, end time.Time) (bool, error) {
	units, err := s.Store.SumUnits(ctx, customerID, start, end)
	if err != nil {
		return false, err
	}
	if units <= 0 {
		return false, nil
	}
	tiers, err := s.Store.Tiers(ctx, customerID)
	if err != nil {
		return false, err
	}
	lines := PriceUnits(units, tiers)
	subtotal := SumAmounts(lines)

	grants, err := s.Store.RemainingCredits(ctx, customerID)
	if err != nil {
		return false, err
	}
	creditLines, applied, left := ApplyCredits(subtotal, grants, len(lines)+1)
	lines = append(lines, creditLines...)

	return s.Store.SaveInvoice(ctx, models.NewInvoice{
		CustomerID:     customerID,
		PeriodStart:    start,
		PeriodEnd:      end,
		SubtotalMicros: subtotal,
		CreditMicros:   applied,
		TotalMicros:    subtotal - applied,
		Lines:          lines,
		Credits:        left,
	})
}

func (s *InvoiceService) List(ctx context.Context, customerID string, offset, limit int) ([]models.Invoice, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.Store.ListInvoices(ctx, customerID, offset, limit)
}

func (s *InvoiceService) Get(ctx context.Context, customerID, invoiceID string) (models.Invoice, error) {
	return s.Store.GetInvoice(ctx, customerID, invoiceID)
}
