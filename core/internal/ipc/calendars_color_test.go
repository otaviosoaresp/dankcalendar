package ipc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/AvengeMedia/dankcalendar/core/ent/account"
	"github.com/AvengeMedia/dankcalendar/core/repo"
)

func newCalendarColorFixture(t *testing.T) (Deps, *repo.Repo) {
	t.Helper()
	ctx := context.Background()
	client, err := repo.OpenMemory(ctx)
	require.NoError(t, err)
	r := repo.New(client)
	t.Cleanup(func() { _ = r.Close() })

	_, err = r.CreateAccount(ctx, repo.CreateAccountInput{ID: "acc", Kind: account.KindCaldav, DisplayName: "Acc"})
	require.NoError(t, err)
	_, err = r.UpsertCalendar(ctx, repo.UpsertCalendarInput{ID: "cal", AccountID: "acc", RemoteID: "remote", Name: "Work", Color: "#ff0000"})
	require.NoError(t, err)

	return Deps{Repo: r, Bus: NewEventBus()}, r
}

func TestCalendarSetColorRejectsInvalidHex(t *testing.T) {
	deps, _ := newCalendarColorFixture(t)

	for name, color := range map[string]string{
		"no hash":     "00ff00",
		"short form":  "#0f0",
		"non hex":     "#zzzzzz",
		"css keyword": "green",
	} {
		t.Run(name, func(t *testing.T) {
			out := routeAndRead(t, Request{ID: 1, Method: "calendars.setColor", Params: map[string]any{"calendarId": "cal", "color": color}}, deps)
			require.NotNil(t, out["error"], "expected %q to be rejected", color)
		})
	}
}

func TestCalendarSetColorOverridesEffectiveColor(t *testing.T) {
	deps, r := newCalendarColorFixture(t)
	ctx := context.Background()

	out := routeAndRead(t, Request{ID: 1, Method: "calendars.setColor", Params: map[string]any{"calendarId": "cal", "color": "#00ff00"}}, deps)
	require.Nil(t, out["error"])

	cal, err := r.GetCalendar(ctx, "cal")
	require.NoError(t, err)
	require.Equal(t, "#00ff00", cal.ColorOverride)
	require.Equal(t, "#ff0000", cal.Color)

	list := routeAndRead(t, Request{ID: 2, Method: "calendars.list"}, deps)
	result, ok := list["result"].([]any)
	require.True(t, ok)
	require.Len(t, result, 1)
	entry := result[0].(map[string]any)
	require.Equal(t, "#00ff00", entry["color"])
	require.Equal(t, "#ff0000", entry["providerColor"])

	out = routeAndRead(t, Request{ID: 3, Method: "calendars.setColor", Params: map[string]any{"calendarId": "cal", "color": ""}}, deps)
	require.Nil(t, out["error"])

	cal, err = r.GetCalendar(ctx, "cal")
	require.NoError(t, err)
	require.Empty(t, cal.ColorOverride)
}
