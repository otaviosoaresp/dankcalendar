package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auth_handler "github.com/AvengeMedia/dankcalendar/core/api/auth"
	"github.com/AvengeMedia/dankcalendar/core/config"
	"github.com/AvengeMedia/dankcalendar/core/ent/account"
	"github.com/AvengeMedia/dankcalendar/core/internal/calendar"
	"github.com/AvengeMedia/dankcalendar/core/internal/icstoken"
	"github.com/AvengeMedia/dankcalendar/core/repo"
)

func startTestHTTP(t *testing.T) (addr string, r *repo.Repo, icsSecret []byte) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	client, err := repo.OpenMemory(context.Background())
	require.NoError(t, err)
	r = repo.New(client)
	t.Cleanup(func() { _ = r.Close() })

	icsSecret, err = icstoken.LoadOrCreateSecret(t.TempDir())
	require.NoError(t, err)

	cfg := &config.Config{APIAddr: "127.0.0.1:0"}
	srv, httpAddr, errCh, err := startHTTP(ctx, cfg, r, calendar.NewRegistry(), nil, auth_handler.NewCallbackBroker(), icsSecret)
	require.NoError(t, err)
	t.Cleanup(func() { shutdownHTTP(srv) })

	go func() {
		if err := <-errCh; err != nil {
			t.Logf("http server: %v", err)
		}
	}()

	return httpAddr, r, icsSecret
}

func TestICSRouteBindsToLoopback(t *testing.T) {
	addr, _, _ := startTestHTTP(t)
	assert.True(t, strings.HasPrefix(addr, "127.0.0.1:"), "expected loopback address, got %q", addr)
}

func TestICSRouteServesKnownCalendar(t *testing.T) {
	addr, r, icsSecret := startTestHTTP(t)
	ctx := context.Background()

	_, err := r.CreateAccount(ctx, repo.CreateAccountInput{ID: "acct-1", Kind: account.KindLocal, DisplayName: "Personal"})
	require.NoError(t, err)
	cal, err := r.UpsertCalendar(ctx, repo.UpsertCalendarInput{ID: "cal-1", AccountID: "acct-1", RemoteID: "dir:cal-1", Name: "Personal"})
	require.NoError(t, err)

	start := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	_, err = r.UpsertEvent(ctx, repo.UpsertEventInput{
		CalendarID: cal.ID,
		UID:        "evt-1",
		Summary:    "Standup",
		Start:      start,
		End:        start.Add(time.Hour),
	})
	require.NoError(t, err)

	token := icstoken.Token(icsSecret, cal.ID)
	resp, err := http.Get("http://" + addr + "/ics/" + token + ".ics")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/calendar")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "BEGIN:VCALENDAR")
	assert.Contains(t, string(body), "SUMMARY:Standup")
}

func TestICSRouteRejectsUnknownToken(t *testing.T) {
	addr, _, _ := startTestHTTP(t)

	resp, err := http.Get("http://" + addr + "/ics/not-a-real-token.ics")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestICSRouteRejectsTokenForDeletedCalendar(t *testing.T) {
	addr, _, icsSecret := startTestHTTP(t)

	token := icstoken.Token(icsSecret, "never-existed")
	resp, err := http.Get("http://" + addr + "/ics/" + token + ".ics")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
