package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"

	"metered-billing/internal/domain"
	"metered-billing/internal/models"
)

type OpsService struct {
	Store domain.OpsStore
}

func (s *OpsService) ListCustomers(ctx context.Context, offset, limit int) ([]models.Customer, error) {
	if limit <= 0 {
		limit = domain.DefaultPageSize
	}
	if limit > domain.MaxOpsPage {
		limit = domain.MaxOpsPage
	}
	return s.Store.ListCustomers(ctx, offset, limit)
}

func (s *OpsService) GetCustomer(ctx context.Context, id string) (models.CustomerDetail, error) {
	return s.Store.GetCustomer(ctx, id)
}

func (s *OpsService) IssueCredit(ctx context.Context, in models.CreditIssue) (models.CreditGrant, bool, error) {
	if in.AmountMicros <= 0 {
		return models.CreditGrant{}, false, domain.ErrAmountMustBePositive
	}
	if in.Reason == "" || in.IdempotencyKey == "" {
		return models.CreditGrant{}, false, domain.ErrReasonAndIdempotency
	}
	if in.Actor == "" {
		in.Actor = domain.ActorOps
	}
	return s.Store.IssueCredit(ctx, in)
}

func (s *OpsService) OverrideLine(ctx context.Context, in models.LineOverride) error {
	if in.Reason == "" {
		return domain.ErrReasonRequired
	}
	if in.Actor == "" {
		in.Actor = domain.ActorOps
	}
	return s.Store.OverrideLine(ctx, in)
}

func (s *OpsService) ApplyPayment(ctx context.Context, ev models.PaymentEvent) (bool, error) {
	if ev.ProviderEventID == "" || ev.InvoiceID == "" {
		return false, domain.ErrEventAndInvoiceID
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
