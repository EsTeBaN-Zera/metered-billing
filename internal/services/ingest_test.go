package services

import (
	"context"
	"testing"
	"time"

	"metered-billing/internal/models"
)

type fakeEvents struct {
	n int
}

func (f *fakeEvents) InsertBatch(ctx context.Context, customerID, apiKeyID string, batch []models.Event) (models.IngestResult, error) {
	f.n += len(batch)
	return models.IngestResult{Inserted: len(batch)}, nil
}

func TestIngestService_rejectsEmpty(t *testing.T) {
	s := &IngestService{Store: &fakeEvents{}}
	_, err := s.Ingest(context.Background(), "c", "k", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIngestService_usesStore(t *testing.T) {
	fake := &fakeEvents{}
	s := &IngestService{Store: fake}
	out, err := s.Ingest(context.Background(), "c", "k", []models.Event{{
		RequestID: "r1",
		Endpoint:  "/x",
		Units:     1,
		Timestamp: time.Date(2026, 3, 15, 14, 0, 0, 0, time.UTC),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Inserted != 1 || fake.n != 1 {
		t.Fatalf("out=%+v n=%d", out, fake.n)
	}
}
