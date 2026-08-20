package postgres

import "metered-billing/internal/domain"

var (
	_ domain.EventStore   = (*Store)(nil)
	_ domain.KeyStore     = (*Store)(nil)
	_ domain.WindowStore  = (*Store)(nil)
	_ domain.InvoiceStore = (*Store)(nil)
	_ domain.UsageStore   = (*Store)(nil)
	_ domain.OpsStore     = (*Store)(nil)
)
