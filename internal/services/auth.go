package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"metered-billing/internal/domain"
	"metered-billing/internal/models"
)

type AuthService struct {
	Keys   domain.KeyStore
	Hasher domain.Hasher
}

func (s *AuthService) FromToken(ctx context.Context, plaintext string) (models.APIKey, error) {
	if s.Keys == nil || s.Hasher == nil {
		return models.APIKey{}, domain.ErrAuthNotConfigured
	}
	if plaintext == "" {
		return models.APIKey{}, domain.ErrUnauthorized
	}
	key, err := s.Keys.LookupByHash(ctx, s.Hasher.Sum(plaintext))
	if err != nil {
		return models.APIKey{}, err
	}
	if key.Revoked {
		return models.APIKey{}, domain.ErrUnauthorized
	}
	return key, nil
}

func NewPlaintext() (plaintext, prefix string, err error) {
	var raw [24]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return "", "", err
	}
	plaintext = domain.APIKeyPrefix + hex.EncodeToString(raw[:])
	prefix = plaintext[:domain.APIKeyPrefixLen]
	return plaintext, prefix, nil
}
