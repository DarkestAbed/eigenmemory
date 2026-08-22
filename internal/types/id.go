package types

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// NewID returns a random 16-byte hex identifier for pages and sources.
func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to a timestamp-based ID is not ideal but avoids panic.
		return strings.ReplaceAll(fmt.Sprintf("%x", b), " ", "")
	}
	return hex.EncodeToString(b)
}
