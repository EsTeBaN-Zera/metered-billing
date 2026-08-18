package postgres

import "metered-billing/internal/services"

var (
	_ services.EventStore  = (*Store)(nil)
	_ services.KeyStore    = (*Store)(nil)
	_ services.WindowStore  = (*Store)(nil)
	_ services.InvoiceStore = (*Store)(nil)
	_ services.UsageStore = (*Store)(nil)
	_ services.OpsStore   = (*Store)(nil)
)
