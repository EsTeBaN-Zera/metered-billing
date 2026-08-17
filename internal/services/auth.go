package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"metered-billing/internal/models"
)

type AuthService struct {
	Keys   KeyStore
	Hasher Hasher
}

func (s *AuthService) FromToken(ctx context.Context, plaintext string) (models.APIKey, error) {
	if s.Keys == nil || s.Hasher == nil {
		return models.APIKey{}, fmt.Errorf("auth is not configured")
	}
	if plaintext == "" {
		return models.APIKey{}, fmt.Errorf("unauthorized")
	}
	key, err := s.Keys.LookupByHash(ctx, s.Hasher.Sum(plaintext))
	if err != nil {
		return models.APIKey{}, err
	}
	if key.Revoked {
		return models.APIKey{}, fmt.Errorf("unauthorized")
	}
	return key, nil
}

func NewPlaintext() (plaintext, prefix string, err error) {
	var raw [24]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return "", "", err
	}
	plaintext = "sk_live_" + hex.EncodeToString(raw[:])
	prefix = plaintext[:20]
	return plaintext, prefix, nil
}
