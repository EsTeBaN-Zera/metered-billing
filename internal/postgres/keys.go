package postgres

import (
	"context"
	"time"

	"metered-billing/internal/models"
)

func (s *Store) LookupByHash(ctx context.Context, keyHash string) (models.APIKey, error) {
	var k models.APIKey
	var revokedAt *time.Time
	err := s.Pool.QueryRow(ctx, `
		SELECT id::text, customer_id::text, revoked_at
		FROM lookup_api_key($1)
	`, keyHash).Scan(&k.ID, &k.CustomerID, &revokedAt)
	if err != nil {
		return models.APIKey{}, err
	}
	k.Revoked = revokedAt != nil
	return k, nil
}
