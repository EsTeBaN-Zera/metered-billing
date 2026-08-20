package controllers

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"metered-billing/internal/domain"
	"metered-billing/internal/models"
)

type ctxKey int

const apiKeyCtx ctxKey = 1

func (c *Controller) requireCustomer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok, ok := bearer(r.Header.Get("Authorization"))
		if !ok {
			writeErr(w, http.StatusUnauthorized, domain.MsgUnauthorized)
			return
		}
		key, err := c.Auth.FromToken(r.Context(), tok)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, domain.MsgUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), apiKeyCtx, key)))
	})
}

func (c *Controller) requireOps(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok, ok := bearer(r.Header.Get("Authorization"))
		if !ok || c.OpsToken == "" {
			writeErr(w, http.StatusUnauthorized, domain.MsgUnauthorized)
			return
		}
		if subtle.ConstantTimeCompare([]byte(tok), []byte(c.OpsToken)) != 1 {
			writeErr(w, http.StatusUnauthorized, domain.MsgUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func apiKeyFrom(r *http.Request) models.APIKey {
	key, _ := r.Context().Value(apiKeyCtx).(models.APIKey)
	return key
}

func bearer(header string) (string, bool) {
	prefix := domain.BearerPrefix
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	tok := strings.TrimSpace(header[len(prefix):])
	return tok, tok != ""
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeErr(w, http.StatusBadRequest, domain.MsgInvalidJSON)
		return false
	}
	return true
}
