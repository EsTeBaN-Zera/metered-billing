package domain

import "errors"

var (
	ErrInvoicePaid          = errors.New(MsgInvoicePaid)
	ErrNotFound             = errors.New(MsgNotFound)
	ErrUnauthorized         = errors.New(MsgUnauthorized)
	ErrEmptyBatch           = errors.New(MsgEmptyBatch)
	ErrBatchTooLarge        = errors.New(MsgBatchTooLarge)
	ErrRequestIDRequired    = errors.New(MsgRequestIDRequired)
	ErrRequestIDTooLong     = errors.New(MsgRequestIDTooLong)
	ErrEndpointRequired     = errors.New(MsgEndpointRequired)
	ErrUnitsMustBePositive  = errors.New(MsgUnitsMustBePositive)
	ErrTimestampRequired    = errors.New(MsgTimestampRequired)
	ErrAmountMustBePositive = errors.New(MsgAmountMustBePositive)
	ErrReasonAndIdempotency = errors.New(MsgReasonAndIdempotency)
	ErrReasonRequired       = errors.New(MsgReasonRequired)
	ErrEventAndInvoiceID    = errors.New(MsgEventAndInvoiceID)
	ErrFromAndToRequired    = errors.New(MsgFromAndToRequired)
	ErrToAfterFrom          = errors.New(MsgToAfterFrom)
	ErrBadCursor            = errors.New(MsgBadCursor)
	ErrEventStoreMissing    = errors.New(MsgEventStoreMissing)
	ErrWindowStoreMissing   = errors.New(MsgWindowStoreMissing)
	ErrInvoiceStoreMissing  = errors.New(MsgInvoiceStoreMissing)
	ErrUsageStoreMissing    = errors.New(MsgUsageStoreMissing)
	ErrAuthNotConfigured    = errors.New(MsgAuthNotConfigured)
)
