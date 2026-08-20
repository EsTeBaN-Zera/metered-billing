package controllers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"metered-billing/internal/models"
)

type stubAuth struct {
	key models.APIKey
	err error
}

func (s stubAuth) FromToken(context.Context, string) (models.APIKey, error) {
	if s.err != nil {
		return models.APIKey{}, s.err
	}
	return s.key, nil
}

func TestRequireCustomer_unauthorizedWithoutBearer(t *testing.T) {
	h := (&Controller{
		Auth: stubAuth{err: errors.New("nope")},
	}).Handler()
	req := httptest.NewRequest(http.MethodGet, "/v1/invoices", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestRequireOps_unauthorizedWithoutToken(t *testing.T) {
	h := (&Controller{OpsToken: "secret"}).Handler()
	req := httptest.NewRequest(http.MethodGet, "/ops/customers", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestHealth_skipsAuth(t *testing.T) {
	h := (&Controller{}).Handler()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
}
