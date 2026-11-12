package secure

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

const FeedTokenPrefix = "rill_"

func RandomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return FeedTokenPrefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

func RandomSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func HashToken(token string) string {
	// Tokens have 256 bits of entropy, so a fast hash is sufficient for
	// session-at-rest storage.
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// MaskToken exposes just enough of a token for an operator to distinguish
// rotations, without making the URL usable.
func MaskToken(token string) string {
	if token == "" {
		return ""
	}
	token = strings.TrimSpace(token)
	if len(token) <= 12 {
		return token[:2] + "…"
	}
	return token[:10] + "…" + token[len(token)-4:]
}
