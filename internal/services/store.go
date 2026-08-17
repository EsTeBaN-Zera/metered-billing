package services

import (
	"context"
	"time"

	"metered-billing/internal/models"
)

type EventStore interface {
	InsertBatch(ctx context.Context, customerID, apiKeyID string, batch []models.Event) (models.IngestResult, error)
}

type KeyStore interface {
	LookupByHash(ctx context.Context, keyHash string) (models.APIKey, error)
}

type Hasher interface {
	Sum(plaintext string) string
}

type Clock interface {
	Now() time.Time
}

type WindowStore interface {
	ProcessDirtyHours(ctx context.Context, limit int) (processed int, err error)
}

type InvoiceStore interface {
	CustomersWithUsage(ctx context.Context, start, end time.Time) ([]string, error)
	SumUnits(ctx context.Context, customerID string, start, end time.Time) (int64, error)
	Tiers(ctx context.Context, customerID string) ([]models.Tier, error)
	RemainingCredits(ctx context.Context, customerID string) ([]models.CreditGrant, error)
	SaveInvoice(ctx context.Context, inv models.NewInvoice) (created bool, err error)
	ListInvoices(ctx context.Context, customerID string, offset, limit int) ([]models.Invoice, error)
	GetInvoice(ctx context.Context, customerID, invoiceID string) (models.Invoice, error)
}

type UsageStore interface {
	ListWindows(ctx context.Context, customerID string, q models.UsageQuery) ([]models.UsageWindow, error)
}

type OpsStore interface {
	ListCustomers(ctx context.Context, offset, limit int) ([]models.Customer, error)
	GetCustomer(ctx context.Context, id string) (models.CustomerDetail, error)
	IssueCredit(ctx context.Context, in models.CreditIssue) (models.CreditGrant, bool, error)
	OverrideLine(ctx context.Context, in models.LineOverride) error
	ApplyPayment(ctx context.Context, ev models.PaymentEvent) (applied bool, err error)
	GetInvoiceByID(ctx context.Context, invoiceID string) (models.Invoice, error)
}
