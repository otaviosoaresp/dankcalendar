// Package icstoken derives the unguessable per-calendar token embedded in an
// ICS subscription URL. The token encodes the calendar id and an HMAC
// signature keyed by a secret persisted once on disk, so a calendar's
// subscription URL stays valid across daemon restarts without a database
// migration.
package icstoken

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const secretFileName = "ics-secret"

// LoadOrCreateSecret reads the HMAC secret from dir, generating a random
// 32-byte one on first use. dir must already exist and be private (the daemon
// data dir, created 0700).
func LoadOrCreateSecret(dir string) ([]byte, error) {
	path := filepath.Join(dir, secretFileName)
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		return data, nil
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, secret, 0o600); err != nil {
		return nil, err
	}
	return secret, nil
}

// Token derives the subscription token for calendarID: the id itself
// (recoverable, not secret) followed by an HMAC signature that only the
// holder of secret could have produced.
func Token(secret []byte, calendarID string) string {
	id := base64.RawURLEncoding.EncodeToString([]byte(calendarID))
	return fmt.Sprintf("%s.%s", id, sign(secret, id))
}

// CalendarID recovers the calendar id from token, reporting false when the
// token is malformed or its signature does not match secret.
func CalendarID(secret []byte, token string) (string, bool) {
	id, sig, ok := strings.Cut(token, ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(sign(secret, id))) {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return "", false
	}
	return string(raw), true
}

func sign(secret []byte, id string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(id))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
