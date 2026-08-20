package services

import (
	"fmt"

	"metered-billing/internal/domain"
	"metered-billing/internal/models"
)

func PriceUnits(units int64, tiers []models.Tier) []models.LineItem {
	var lines []models.LineItem
	pos := 1
	for _, t := range tiers {
		if units <= t.From {
			break
		}
		hi := units
		if t.To != nil && *t.To < hi {
			hi = *t.To
		}
		qty := hi - t.From
		if qty <= 0 {
			continue
		}
		lines = append(lines, models.LineItem{
			Kind:         domain.LineKindTier,
			Description:  tierLabel(t, qty),
			Quantity:     qty,
			UnitMicros:   t.PriceMicros,
			AmountMicros: qty * t.PriceMicros,
			Position:     pos,
		})
		pos++
	}
	return lines
}

func SumAmounts(lines []models.LineItem) int64 {
	var n int64
	for _, l := range lines {
		n += l.AmountMicros
	}
	return n
}

func ApplyCredits(subtotal int64, grants []models.CreditGrant, nextPos int) (lines []models.LineItem, applied int64, left []models.CreditGrant) {
	left = make([]models.CreditGrant, len(grants))
	copy(left, grants)
	need := subtotal
	pos := nextPos
	for i := range left {
		if need == 0 {
			break
		}
		take := left[i].RemainingMicros
		if take > need {
			take = need
		}
		if take == 0 {
			continue
		}
		lines = append(lines, models.LineItem{
			Kind:         domain.LineKindCredit,
			Description:  domain.CreditLineDesc,
			Quantity:     0,
			UnitMicros:   0,
			AmountMicros: -take,
			Position:     pos,
		})
		pos++
		left[i].RemainingMicros -= take
		applied += take
		need -= take
	}
	return lines, applied, left
}

func tierLabel(t models.Tier, qty int64) string {
	if t.To == nil {
		return fmt.Sprintf(domain.FmtTierOpen, qty, t.From)
	}
	return fmt.Sprintf(domain.FmtTierClosed, qty, t.From, *t.To)
}
