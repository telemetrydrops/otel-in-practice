package telemetry

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// HashEmail returns a SHA-256 hex digest of the lower-cased email address.
// Using a consistent one-way hash allows correlation across signals (traces,
// metrics, logs) without storing the raw PII value.
func HashEmail(email string) string {
	h := sha256.Sum256([]byte(strings.ToLower(email)))
	return fmt.Sprintf("%x", h)
}
