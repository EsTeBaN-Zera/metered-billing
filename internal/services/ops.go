package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"metered-billing/internal/models"
)

type OpsService struct {
	Store OpsStore
}

func (s *OpsService) ListCustomers(ctx context.Context, offset, limit int) ([]models.Customer, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	return s.Store.ListCustomers(ctx, offset, limit)
}

func (s *OpsService) GetCustomer(ctx context.Context, id string) (models.CustomerDetail, error) {
	return s.Store.GetCustomer(ctx, id)
}

func (s *OpsService) IssueCredit(ctx context.Context, in models.CreditIssue) (models.CreditGrant, bool, error) {
	if in.AmountMicros <= 0 {
		return models.CreditGrant{}, false, fmt.Errorf("amount must be > 0")
	}
	if in.Reason == "" || in.IdempotencyKey == "" {
		return models.CreditGrant{}, false, fmt.Errorf("reason and Idempotency-Key are required")
	}
	if in.Actor == "" {
		in.Actor = "ops"
	}
	return s.Store.IssueCredit(ctx, in)
}

func (s *OpsService) OverrideLine(ctx context.Context, in models.LineOverride) error {
	if in.Reason == "" {
		return fmt.Errorf("reason is required")
	}
	if in.Actor == "" {
		in.Actor = "ops"
	}
	return s.Store.OverrideLine(ctx, in)
}

func (s *OpsService) ApplyPayment(ctx context.Context, ev models.PaymentEvent) (bool, error) {
	if ev.ProviderEventID == "" || ev.InvoiceID == "" {
		return false, fmt.Errorf("event_id and invoice_id are required")
	}
	return s.Store.ApplyPayment(ctx, ev)
}

func (s *OpsService) GetInvoice(ctx context.Context, invoiceID string) (models.Invoice, error) {
	return s.Store.GetInvoiceByID(ctx, invoiceID)
}

type HMACVerifier struct {
	Secret string
}

func (v HMACVerifier) Valid(body []byte, signatureHex string) bool {
	mac := hmac.New(sha256.New, []byte(v.Secret))
	_, _ = mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	got, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false
	}
	wantB, err := hex.DecodeString(want)
	if err != nil {
		return false
	}
	return hmac.Equal(got, wantB)
}
