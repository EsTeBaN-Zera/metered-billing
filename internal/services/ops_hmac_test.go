package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestHMACVerifier(t *testing.T) {
	secret := "s3cret"
	body := []byte(`{"event_id":"e1"}`)
	v := HMACVerifier{Secret: secret}
	if v.Valid(body, "deadbeef") {
		t.Fatal("bad sig accepted")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	good := hex.EncodeToString(mac.Sum(nil))
	if !v.Valid(body, good) {
		t.Fatal("good sig rejected")
	}
}
