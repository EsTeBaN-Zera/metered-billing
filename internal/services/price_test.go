package services

import (
	"testing"

	"metered-billing/internal/models"
)

func TestPriceUnits_standardPlan(t *testing.T) {
	to10k := int64(10000)
	to100k := int64(100000)
	tiers := []models.Tier{
		{From: 0, To: &to10k, PriceMicros: 0, SortOrder: 1},
		{From: 10000, To: &to100k, PriceMicros: 1000, SortOrder: 2},
		{From: 100000, To: nil, PriceMicros: 500, SortOrder: 3},
	}

	lines := PriceUnits(150000, tiers)
	if len(lines) != 3 {
		t.Fatalf("lines=%d", len(lines))
	}
	if lines[0].AmountMicros != 0 || lines[0].Quantity != 10000 {
		t.Fatalf("free tier %+v", lines[0])
	}
	if lines[1].AmountMicros != 90_000_000 || lines[1].Quantity != 90000 {
		t.Fatalf("mid tier %+v", lines[1])
	}
	if lines[2].AmountMicros != 25_000_000 || lines[2].Quantity != 50000 {
		t.Fatalf("rest tier %+v", lines[2])
	}
	if SumAmounts(lines) != 115_000_000 {
		t.Fatalf("subtotal %d", SumAmounts(lines))
	}
}

func TestApplyCredits_partial(t *testing.T) {
	grants := []models.CreditGrant{{ID: "g1", RemainingMicros: 20_000_000}}
	lines, applied, left := ApplyCredits(115_000_000, grants, 4)
	if applied != 20_000_000 || len(lines) != 1 || left[0].RemainingMicros != 0 {
		t.Fatalf("applied=%d lines=%+v left=%+v", applied, lines, left)
	}
}
