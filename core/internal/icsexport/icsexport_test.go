package icsexport_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	ical "github.com/emersion/go-ical"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AvengeMedia/dankcalendar/core/ent/account"
	"github.com/AvengeMedia/dankcalendar/core/ent/event"
	"github.com/AvengeMedia/dankcalendar/core/internal/calendar"
	"github.com/AvengeMedia/dankcalendar/core/internal/icsexport"
	"github.com/AvengeMedia/dankcalendar/core/internal/icsimport"
	"github.com/AvengeMedia/dankcalendar/core/internal/providers/icalconv"
	"github.com/AvengeMedia/dankcalendar/core/repo"
)

func newTestRepo(t *testing.T) (*repo.Repo, context.Context) {
	t.Helper()
	ctx := context.Background()
	client, err := repo.OpenMemory(ctx)
	require.NoError(t, err)

	r := repo.New(client)
	t.Cleanup(func() { _ = r.Close() })
	return r, ctx
}

func seedCalendar(t *testing.T, r *repo.Repo, ctx context.Context) string {
	t.Helper()
	_, err := r.CreateAccount(ctx, repo.CreateAccountInput{
		ID:          "acct-1",
		Kind:        account.KindLocal,
		DisplayName: "Personal",
	})
	require.NoError(t, err)

	cal, err := r.UpsertCalendar(ctx, repo.UpsertCalendarInput{
		ID:        "cal-1",
		AccountID: "acct-1",
		RemoteID:  "dir:cal-1",
		Name:      "Personal",
	})
	require.NoError(t, err)
	return cal.ID
}

func TestCalendarExportsTimedAllDayAndRecurringEvents(t *testing.T) {
	r, ctx := newTestRepo(t)
	calID := seedCalendar(t, r, ctx)

	timedStart := time.Date(2026, 1, 5, 14, 0, 0, 0, time.UTC)
	_, err := r.UpsertEvent(ctx, repo.UpsertEventInput{
		CalendarID: calID,
		UID:        "timed-1",
		Summary:    "Timed event",
		Start:      timedStart,
		End:        timedStart.Add(time.Hour),
		StartTZ:    "America/New_York",
		EndTZ:      "America/New_York",
	})
	require.NoError(t, err)

	allDayStart := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	_, err = r.UpsertEvent(ctx, repo.UpsertEventInput{
		CalendarID: calID,
		UID:        "allday-1",
		Summary:    "All day event",
		Start:      allDayStart,
		End:        allDayStart.Add(24 * time.Hour),
		AllDay:     true,
	})
	require.NoError(t, err)

	seriesStart := time.Date(2026, 1, 12, 9, 0, 0, 0, time.UTC)
	rec := &calendar.Recurrence{RRule: []string{"FREQ=WEEKLY;COUNT=5"}}
	_, err = r.UpsertEvent(ctx, repo.UpsertEventInput{
		CalendarID: calID,
		UID:        "series-1",
		Summary:    "Weekly sync",
		Start:      seriesStart,
		End:        seriesStart.Add(time.Hour),
		Recurrence: rec.ToMap(),
	})
	require.NoError(t, err)

	overrideOriginal := seriesStart.AddDate(0, 0, 7)
	overrideStart := overrideOriginal.Add(time.Hour)
	_, err = r.UpsertEvent(ctx, repo.UpsertEventInput{
		CalendarID:    calID,
		UID:           "series-1/20260119T090000Z",
		Summary:       "Weekly sync (moved)",
		Start:         overrideStart,
		End:           overrideStart.Add(time.Hour),
		RecurringID:   "series-1",
		OriginalStart: overrideOriginal,
	})
	require.NoError(t, err)

	ics, err := icsexport.Calendar(ctx, r, calID)
	require.NoError(t, err)

	// The exported document round-trips through the existing icsimport parser,
	// which surfaces every master event (it deliberately skips occurrence
	// overrides, as those only make sense alongside their series master).
	doc, err := icsimport.Parse([]byte(ics))
	require.NoError(t, err)
	require.Len(t, doc.Events, 3)

	byUID := map[string]calendar.Event{}
	for _, ev := range doc.Events {
		byUID[ev.UID] = ev
	}

	timed, ok := byUID["timed-1"]
	require.True(t, ok)
	assert.False(t, timed.AllDay)
	assert.Equal(t, "America/New_York", timed.StartTimeZone)
	assert.True(t, timed.Start.Equal(timedStart))

	allDay, ok := byUID["allday-1"]
	require.True(t, ok)
	assert.True(t, allDay.AllDay)

	series, ok := byUID["series-1"]
	require.True(t, ok)
	require.NotNil(t, series.Recurrence)
	assert.Equal(t, []string{"FREQ=WEEKLY;COUNT=5"}, series.Recurrence.RRule)

	// The occurrence override is intentionally absent from icsimport's output;
	// verify it separately at the component level, where it anchors back to
	// the series master via RECURRENCE-ID.
	dec := ical.NewDecoder(bytes.NewReader([]byte(ics)))
	vcal, err := dec.Decode()
	require.NoError(t, err)

	tz := icalconv.NewTZResolver(vcal, "")
	foundOverride := false
	for _, comp := range vcal.Events() {
		if icalconv.ComponentUID(comp.Component) != "series-1" {
			continue
		}
		if comp.Props.Get(ical.PropRecurrenceID) == nil {
			continue
		}
		ev, ok := icalconv.EventFromComponent("", comp.Component, tz)
		require.True(t, ok)
		assert.Equal(t, "series-1", ev.RecurringID)
		assert.True(t, ev.OriginalStart.Equal(overrideOriginal))
		assert.Equal(t, "Weekly sync (moved)", ev.Summary)
		foundOverride = true
	}
	assert.True(t, foundOverride, "expected an exception VEVENT anchored to series-1")
}

// TestCalendarFoldsCancelledInstanceIntoExdate covers a deleted occurrence of
// a synced Google series: the sync engine stores it as an exception row with
// only id, RecurringID, OriginalStart, and a cancelled status (Google guarantees
// nothing else for it), so Start/End are zero. Exporting that row as its own
// VEVENT would hand subscribers a blank event in year 0001 that never replaces
// the real occurrence; it must instead exclude the occurrence via EXDATE.
func TestCalendarFoldsCancelledInstanceIntoExdate(t *testing.T) {
	r, ctx := newTestRepo(t)
	calID := seedCalendar(t, r, ctx)

	seriesStart := time.Date(2026, 1, 12, 9, 0, 0, 0, time.UTC)
	rec := &calendar.Recurrence{RRule: []string{"FREQ=WEEKLY;COUNT=5"}}
	_, err := r.UpsertEvent(ctx, repo.UpsertEventInput{
		CalendarID: calID,
		UID:        "series-2",
		Summary:    "Weekly sync",
		Start:      seriesStart,
		End:        seriesStart.Add(time.Hour),
		Recurrence: rec.ToMap(),
	})
	require.NoError(t, err)

	cancelledOriginal := seriesStart.AddDate(0, 0, 7)
	_, err = r.UpsertEvent(ctx, repo.UpsertEventInput{
		CalendarID:    calID,
		UID:           "series-2/20260119T090000Z",
		RecurringID:   "series-2",
		OriginalStart: cancelledOriginal,
		Status:        event.StatusCancelled,
	})
	require.NoError(t, err)

	ics, err := icsexport.Calendar(ctx, r, calID)
	require.NoError(t, err)

	// No standalone VEVENT for the cancelled instance, and no DTSTART in year 1.
	assert.NotContains(t, ics, "00010101")
	dec := ical.NewDecoder(bytes.NewReader([]byte(ics)))
	vcal, err := dec.Decode()
	require.NoError(t, err)
	for _, comp := range vcal.Events() {
		assert.Nil(t, comp.Props.Get(ical.PropRecurrenceID), "cancelled instance must not be written as its own VEVENT")
	}

	// The master's RRULE now excludes the cancelled occurrence via EXDATE, so
	// icsimport's expansion (via the recurrence package) skips it.
	doc, err := icsimport.Parse([]byte(ics))
	require.NoError(t, err)
	require.Len(t, doc.Events, 1)
	series := doc.Events[0]
	require.NotNil(t, series.Recurrence)
	assert.Equal(t, []string{cancelledOriginal.UTC().Format("20060102T150405Z")}, series.Recurrence.ExDate)
}

func TestCalendarReturnsNotFoundForUnknownCalendar(t *testing.T) {
	r, ctx := newTestRepo(t)

	_, err := icsexport.Calendar(ctx, r, "missing")
	require.Error(t, err)
	assert.True(t, repo.IsNotFound(err))
}
