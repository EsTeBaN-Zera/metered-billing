package models

import "time"

type Customer struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type CustomerDetail struct {
	Customer
	Anomaly    bool      `json:"anomaly"`
	TodayUnits int64     `json:"today_units"`
	Avg30Units float64   `json:"avg_30_units"`
	Invoices   []Invoice `json:"invoices"`
}

type CreditIssue struct {
	CustomerID     string
	AmountMicros   int64
	Reason         string
	Actor          string
	IdempotencyKey string
}

type LineOverride struct {
	InvoiceID    string
	LineID       string
	AmountMicros int64
	Reason       string
	Actor        string
}

type PaymentEvent struct {
	ProviderEventID string
	InvoiceID       string
}
