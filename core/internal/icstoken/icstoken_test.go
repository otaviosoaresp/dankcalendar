package icstoken_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AvengeMedia/dankcalendar/core/internal/icstoken"
)

func TestTokenRoundTrips(t *testing.T) {
	secret, err := icstoken.LoadOrCreateSecret(t.TempDir())
	require.NoError(t, err)

	token := icstoken.Token(secret, "cal-123")

	id, ok := icstoken.CalendarID(secret, token)
	assert.True(t, ok)
	assert.Equal(t, "cal-123", id)
}

func TestCalendarIDRejectsTamperedToken(t *testing.T) {
	secret, err := icstoken.LoadOrCreateSecret(t.TempDir())
	require.NoError(t, err)

	token := icstoken.Token(secret, "cal-123")
	tampered := token[:len(token)-1] + "x"

	_, ok := icstoken.CalendarID(secret, tampered)
	assert.False(t, ok)
}

func TestCalendarIDRejectsWrongSecret(t *testing.T) {
	secretA, err := icstoken.LoadOrCreateSecret(t.TempDir())
	require.NoError(t, err)
	secretB, err := icstoken.LoadOrCreateSecret(t.TempDir())
	require.NoError(t, err)

	token := icstoken.Token(secretA, "cal-123")

	_, ok := icstoken.CalendarID(secretB, token)
	assert.False(t, ok)
}

func TestCalendarIDRejectsMalformedToken(t *testing.T) {
	secret, err := icstoken.LoadOrCreateSecret(t.TempDir())
	require.NoError(t, err)

	_, ok := icstoken.CalendarID(secret, "not-a-valid-token")
	assert.False(t, ok)
}

func TestLoadOrCreateSecretPersists(t *testing.T) {
	dir := t.TempDir()

	first, err := icstoken.LoadOrCreateSecret(dir)
	require.NoError(t, err)

	second, err := icstoken.LoadOrCreateSecret(dir)
	require.NoError(t, err)

	assert.Equal(t, first, second)
	assert.FileExists(t, filepath.Join(dir, "ics-secret"))
}
