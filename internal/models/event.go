package models

import "time"

type Event struct {
	RequestID string
	Endpoint  string
	Units     int64
	Timestamp time.Time
}

type IngestResult struct {
	Inserted   int
	Duplicates int
}
