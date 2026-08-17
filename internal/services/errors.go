package services

import "errors"

var (
	ErrInvoicePaid = errors.New("invoice is paid")
	ErrNotFound    = errors.New("not found")
)
