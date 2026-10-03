package ipc

import (
	"context"
	"os"

	"github.com/AvengeMedia/dankcalendar/core/internal/icsexport"
	"github.com/AvengeMedia/dankcalendar/core/internal/icstoken"
)

func handleCalendarExport(ctx context.Context, w *ConnWriter, req Request, deps Deps) {
	calendarID := ParamString(req.Params, "calendarId")
	if calendarID == "" {
		RespondError(w, req.ID, "calendarId is required")
		return
	}
	ics, err := icsexport.Calendar(ctx, deps.Repo, calendarID)
	if err != nil {
		RespondError(w, req.ID, err.Error())
		return
	}
	Respond(w, req.ID, ics)
}

func handleCalendarExportToFile(ctx context.Context, w *ConnWriter, req Request, deps Deps) {
	calendarID := ParamString(req.Params, "calendarId")
	path := ParamString(req.Params, "path")
	if calendarID == "" || path == "" {
		RespondError(w, req.ID, "calendarId and path are required")
		return
	}
	ics, err := icsexport.Calendar(ctx, deps.Repo, calendarID)
	if err != nil {
		RespondError(w, req.ID, err.Error())
		return
	}
	if err := os.WriteFile(path, []byte(ics), 0o644); err != nil {
		RespondError(w, req.ID, err.Error())
		return
	}
	Respond(w, req.ID, map[string]any{"path": path})
}

func handleCalendarIcsLink(ctx context.Context, w *ConnWriter, req Request, deps Deps) {
	calendarID := ParamString(req.Params, "calendarId")
	if calendarID == "" {
		RespondError(w, req.ID, "calendarId is required")
		return
	}
	if _, err := deps.Repo.GetCalendar(ctx, calendarID); err != nil {
		RespondError(w, req.ID, err.Error())
		return
	}

	token := icstoken.Token(deps.IcsSecret, calendarID)
	link, err := redirectURLForHost(deps.HTTPAddr, "", "/ics/"+token+".ics")
	if err != nil {
		RespondError(w, req.ID, err.Error())
		return
	}
	Respond(w, req.ID, map[string]any{"url": link})
}
