package models

import "time"

type Tier struct {
	From        int64
	To          *int64 // nil = no cap
	PriceMicros int64
	SortOrder   int
}

type LineItem struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Description  string `json:"description"`
	Quantity     int64  `json:"quantity"`
	UnitMicros   int64  `json:"unit_micros"`
	AmountMicros int64  `json:"amount_micros"`
	Position     int    `json:"position"`
}

type CreditGrant struct {
	ID              string
	RemainingMicros int64
}

type NewInvoice struct {
	CustomerID     string
	PeriodStart    time.Time
	PeriodEnd      time.Time
	SubtotalMicros int64
	CreditMicros   int64
	TotalMicros    int64
	Lines          []LineItem
	Credits        []CreditGrant
}

type Invoice struct {
	ID             string     `json:"id"`
	CustomerID     string     `json:"customer_id"`
	PeriodStart    time.Time  `json:"period_start"`
	PeriodEnd      time.Time  `json:"period_end"`
	Status         string     `json:"status"`
	SubtotalMicros int64      `json:"subtotal_micros"`
	CreditMicros   int64      `json:"credit_micros"`
	TotalMicros    int64      `json:"total_micros"`
	IssuedAt       time.Time  `json:"issued_at"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	Lines          []LineItem `json:"line_items,omitempty"`
}
