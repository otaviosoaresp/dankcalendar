// Package icsexport serializes a stored calendar's events into a single
// iCalendar document, shared by the CLI export command and the HTTP
// subscription route so both emit identical output.
package icsexport

import (
	"bytes"
	"context"
	"time"

	ical "github.com/emersion/go-ical"

	cal "github.com/AvengeMedia/dankcalendar/core/internal/calendar"
	"github.com/AvengeMedia/dankcalendar/core/internal/eventconv"
	"github.com/AvengeMedia/dankcalendar/core/internal/providers/icalconv"
	"github.com/AvengeMedia/dankcalendar/core/repo"
)

// Calendar renders every event in calendarID as a VCALENDAR document. A
// recurring series is written as its master VEVENT plus one VEVENT per
// overridden occurrence, anchored back to the master with RECURRENCE-ID. A
// cancelled occurrence (a deleted instance of a synced series) carries no
// usable DTSTART from providers like Google, which only guarantee id,
// recurringEventId, originalStartTime and status for it; writing it as its
// own VEVENT would produce a blank event in year 0001, so it is folded into
// the master's EXDATE instead.
func Calendar(ctx context.Context, r *repo.Repo, calendarID string) (string, error) {
	if _, err := r.GetCalendar(ctx, calendarID); err != nil {
		return "", err
	}

	rows, _, err := r.ListEvents(ctx, repo.ListEventsParams{
		Filter: repo.EventFilter{CalendarIDs: []string{calendarID}},
	})
	if err != nil {
		return "", err
	}

	var events []cal.Event
	cancelled := map[string][]time.Time{}
	for _, e := range rows {
		ev := eventconv.FromEnt(e)
		if ev.RecurringID != "" && ev.Status == cal.EventCancelled {
			cancelled[ev.RecurringID] = append(cancelled[ev.RecurringID], ev.OriginalStart)
			continue
		}
		events = append(events, ev)
	}

	doc := icalconv.NewCalendar()
	for i := range events {
		ev := &events[i]
		uid := ev.UID
		if ev.RecurringID != "" {
			uid = ev.RecurringID
		}
		if ev.RecurringID == "" {
			if dates := cancelled[ev.UID]; len(dates) > 0 {
				addExDates(ev, dates)
			}
		}
		doc.Children = append(doc.Children, icalconv.BuildEvent(ev, uid).Component)
	}

	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(doc); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// addExDates excludes each cancelled instance from ev's RRULE expansion.
func addExDates(ev *cal.Event, dates []time.Time) {
	rec := cal.Recurrence{}
	if ev.Recurrence != nil {
		rec = *ev.Recurrence
	}
	for _, d := range dates {
		if ev.AllDay {
			rec.ExDate = append(rec.ExDate, d.UTC().Format("20060102"))
			continue
		}
		rec.ExDate = append(rec.ExDate, d.UTC().Format("20060102T150405Z"))
	}
	ev.Recurrence = &rec
}
