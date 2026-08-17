package services

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type PepperHash struct {
	Pepper string
}

func (h PepperHash) Sum(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext + h.Pepper))
	return hex.EncodeToString(sum[:])
}

type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now().UTC()
}
