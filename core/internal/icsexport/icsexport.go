// Package icsexport serializes a stored calendar's events into a single
// iCalendar document, shared by the CLI export command and the HTTP
// subscription route so both emit identical output.
package icsexport

import (
	"bytes"
	"context"

	ical "github.com/emersion/go-ical"

	"github.com/AvengeMedia/dankcalendar/core/internal/eventconv"
	"github.com/AvengeMedia/dankcalendar/core/internal/providers/icalconv"
	"github.com/AvengeMedia/dankcalendar/core/repo"
)

// Calendar renders every event in calendarID as a VCALENDAR document. A
// recurring series is written as its master VEVENT plus one VEVENT per
// overridden occurrence, anchored back to the master with RECURRENCE-ID.
func Calendar(ctx context.Context, r *repo.Repo, calendarID string) (string, error) {
	if _, err := r.GetCalendar(ctx, calendarID); err != nil {
		return "", err
	}

	events, _, err := r.ListEvents(ctx, repo.ListEventsParams{
		Filter: repo.EventFilter{CalendarIDs: []string{calendarID}},
	})
	if err != nil {
		return "", err
	}

	doc := icalconv.NewCalendar()
	for _, e := range events {
		ev := eventconv.FromEnt(e)
		uid := ev.UID
		if ev.RecurringID != "" {
			uid = ev.RecurringID
		}
		doc.Children = append(doc.Children, icalconv.BuildEvent(&ev, uid).Component)
	}

	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(doc); err != nil {
		return "", err
	}
	return buf.String(), nil
}
