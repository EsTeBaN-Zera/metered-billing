package models

import "time"

type DirtyHour struct {
	CustomerID string
	APIKeyID   string
	Hour       time.Time
	Gen        int64
}

type UsageWindow struct {
	CustomerID string
	APIKeyID   string
	Hour       time.Time
	Units      int64
}

type UsageQuery struct {
	From      time.Time
	To        time.Time
	APIKeyID  string // optional
	Cursor    string
	Limit     int
	AfterHour time.Time
	AfterKey  string
}

type UsagePage struct {
	Windows    []UsageWindow
	NextCursor string
}
