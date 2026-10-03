package ipc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/AvengeMedia/dankcalendar/core/ent/account"
	"github.com/AvengeMedia/dankcalendar/core/internal/icstoken"
	"github.com/AvengeMedia/dankcalendar/core/repo"
)

func newCalendarExportFixture(t *testing.T) Deps {
	t.Helper()
	ctx := context.Background()
	client, err := repo.OpenMemory(ctx)
	require.NoError(t, err)
	r := repo.New(client)
	t.Cleanup(func() { _ = r.Close() })

	_, err = r.CreateAccount(ctx, repo.CreateAccountInput{ID: "acc", Kind: account.KindLocal, DisplayName: "Acc"})
	require.NoError(t, err)
	_, err = r.UpsertCalendar(ctx, repo.UpsertCalendarInput{ID: "cal", AccountID: "acc", RemoteID: "dir:cal", Name: "Work"})
	require.NoError(t, err)

	start := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	_, err = r.UpsertEvent(ctx, repo.UpsertEventInput{
		CalendarID: "cal",
		UID:        "evt-1",
		Summary:    "Kickoff",
		Start:      start,
		End:        start.Add(time.Hour),
	})
	require.NoError(t, err)

	secret, err := icstoken.LoadOrCreateSecret(t.TempDir())
	require.NoError(t, err)

	return Deps{Repo: r, Bus: NewEventBus(), HTTPAddr: "127.0.0.1:47621", IcsSecret: secret}
}

func TestCalendarExportReturnsICS(t *testing.T) {
	deps := newCalendarExportFixture(t)

	out := routeAndRead(t, Request{ID: 1, Method: "calendars.export", Params: map[string]any{"calendarId": "cal"}}, deps)
	require.Nil(t, out["error"])

	ics, ok := out["result"].(string)
	require.True(t, ok)
	require.Contains(t, ics, "BEGIN:VCALENDAR")
	require.Contains(t, ics, "SUMMARY:Kickoff")
}

func TestCalendarExportUnknownCalendar(t *testing.T) {
	deps := newCalendarExportFixture(t)

	out := routeAndRead(t, Request{ID: 1, Method: "calendars.export", Params: map[string]any{"calendarId": "missing"}}, deps)
	require.NotNil(t, out["error"])
}

func TestCalendarExportToFileWritesFile(t *testing.T) {
	deps := newCalendarExportFixture(t)
	dest := filepath.Join(t.TempDir(), "work.ics")

	out := routeAndRead(t, Request{ID: 1, Method: "calendars.exportToFile", Params: map[string]any{"calendarId": "cal", "path": dest}}, deps)
	require.Nil(t, out["error"])

	data, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Contains(t, string(data), "SUMMARY:Kickoff")
}

func TestCalendarIcsLinkReturnsURL(t *testing.T) {
	deps := newCalendarExportFixture(t)

	out := routeAndRead(t, Request{ID: 1, Method: "calendars.icsLink", Params: map[string]any{"calendarId": "cal"}}, deps)
	require.Nil(t, out["error"])

	result, ok := out["result"].(map[string]any)
	require.True(t, ok)
	url, _ := result["url"].(string)
	require.Contains(t, url, "http://127.0.0.1:47621/ics/")

	token := url[len("http://127.0.0.1:47621/ics/") : len(url)-len(".ics")]
	calendarID, ok := icstoken.CalendarID(deps.IcsSecret, token)
	require.True(t, ok)
	require.Equal(t, "cal", calendarID)
}

func TestCalendarIcsLinkUnknownCalendar(t *testing.T) {
	deps := newCalendarExportFixture(t)

	out := routeAndRead(t, Request{ID: 1, Method: "calendars.icsLink", Params: map[string]any{"calendarId": "missing"}}, deps)
	require.NotNil(t, out["error"])
}
