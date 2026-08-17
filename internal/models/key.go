package models

type APIKey struct {
	ID         string
	CustomerID string
	Revoked    bool
}
