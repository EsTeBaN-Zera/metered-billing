package controllers

import (
	"net/http"

	"metered-billing/internal/domain"
)

type Controller struct {
	Auth         domain.Authenticator
	Ingest       domain.Ingester
	Usage        domain.UsageLister
	Invoices     domain.InvoiceViewer
	Ops          domain.OpsAPI
	DB           domain.Pinger
	APIVersion   string
	OpsToken     string
	WebhookCheck domain.SignatureCheck
}

func (c *Controller) apiPrefix() string {
	v := c.APIVersion
	if v == "" {
		v = domain.DefaultAPIVersion
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
	mux.HandleFunc("POST /webhooks/payments", c.webhookPayment)

	customer := func(h http.HandlerFunc) http.Handler {
		return c.requireCustomer(h)
	}
	mux.Handle("POST /"+p+"/events", customer(c.postEvents))
	mux.Handle("GET /"+p+"/usage", customer(c.getUsage))
	mux.Handle("GET /"+p+"/invoices/{id}", customer(c.getInvoice))
	mux.Handle("GET /"+p+"/invoices", customer(c.listInvoices))

	ops := func(h http.HandlerFunc) http.Handler {
		return c.requireOps(h)
	}
	mux.Handle("GET /ops/customers/{id}", ops(c.opsGetCustomer))
	mux.Handle("GET /ops/customers", ops(c.opsListCustomers))
	mux.Handle("POST /ops/customers/{id}/credits", ops(c.opsCredit))
	mux.Handle("GET /ops/invoices/{id}", ops(c.opsGetInvoice))
	mux.Handle("PATCH /ops/invoices/{id}/line-items/{lineId}", ops(c.opsOverride))

	return mux
}
