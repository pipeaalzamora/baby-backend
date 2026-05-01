package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
)

// randomHex generates a cryptographically random hex string of n bytes.
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// parseJSON unmarshals a JSON string into v.
func parseJSON(s string, v interface{}) error {
	return json.Unmarshal([]byte(s), v)
}

// coalesce returns the first non-empty string.
func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
