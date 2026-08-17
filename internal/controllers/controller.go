package controllers

import (
	"context"
	"net/http"

	"metered-billing/internal/models"
)

type Ingester interface {
	Ingest(ctx context.Context, customerID, apiKeyID string, batch []models.Event) (models.IngestResult, error)
}

type Authenticator interface {
	FromToken(ctx context.Context, plaintext string) (models.APIKey, error)
}

type Pinger interface {
	Ping(ctx context.Context) error
}

type UsageLister interface {
	List(ctx context.Context, customerID string, q models.UsageQuery) (models.UsagePage, error)
}

type InvoiceViewer interface {
	List(ctx context.Context, customerID string, offset, limit int) ([]models.Invoice, error)
	Get(ctx context.Context, customerID, invoiceID string) (models.Invoice, error)
}

type OpsAPI interface {
	ListCustomers(ctx context.Context, offset, limit int) ([]models.Customer, error)
	GetCustomer(ctx context.Context, id string) (models.CustomerDetail, error)
	IssueCredit(ctx context.Context, in models.CreditIssue) (models.CreditGrant, bool, error)
	OverrideLine(ctx context.Context, in models.LineOverride) error
	ApplyPayment(ctx context.Context, ev models.PaymentEvent) (bool, error)
	GetInvoice(ctx context.Context, invoiceID string) (models.Invoice, error)
}

type SignatureCheck interface {
	Valid(body []byte, signatureHex string) bool
}

type Controller struct {
	Auth         Authenticator
	Ingest       Ingester
	Usage        UsageLister
	Invoices     InvoiceViewer
	Ops          OpsAPI
	DB           Pinger
	APIVersion   string
	OpsToken     string
	WebhookCheck SignatureCheck
}

func (c *Controller) apiPrefix() string {
	v := c.APIVersion
	if v == "" {
		v = "v1"
	}
	if v[0] == '/' {
		v = v[1:]
	}
	return v
}

func (c *Controller) Handler() http.Handler {
	p := c.apiPrefix()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", c.health)
	mux.HandleFunc("POST /"+p+"/events", c.postEvents)
	mux.HandleFunc("GET /"+p+"/usage", c.getUsage)
	mux.HandleFunc("GET /"+p+"/invoices/{id}", c.getInvoice)
	mux.HandleFunc("GET /"+p+"/invoices", c.listInvoices)
	mux.HandleFunc("GET /ops/customers/{id}", c.opsGetCustomer)
	mux.HandleFunc("GET /ops/customers", c.opsListCustomers)
	mux.HandleFunc("POST /ops/customers/{id}/credits", c.opsCredit)
	mux.HandleFunc("GET /ops/invoices/{id}", c.opsGetInvoice)
	mux.HandleFunc("PATCH /ops/invoices/{id}/line-items/{lineId}", c.opsOverride)
	mux.HandleFunc("POST /webhooks/payments", c.webhookPayment)
	return mux
}
