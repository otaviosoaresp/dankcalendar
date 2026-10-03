package main

import (
	"context"
	"os"
	"testing"

	"github.com/AvengeMedia/dankgo/ipc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// remindersCall dials the daemon's unix socket and must hand back the result
// in a form callers can type-assert directly (e.g. export.go's
// result.(string)). A prior bug returned the raw *any the wire client wraps
// results in, so that assertion always failed.
func TestRemindersCallUnwrapsStringResult(t *testing.T) {
	// A short, fixed-depth dir: the test's own t.TempDir() nests under the
	// test name and can exceed the unix socket path length limit.
	runtimeDir, err := os.MkdirTemp("", "dcal-ipc")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(runtimeDir) })
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)

	handler := func(_ context.Context, w *ipc.ConnWriter, req ipc.Request, _ *ipc.Subscriber) {
		ipc.Respond(w, req.ID, "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n")
	}
	srv := ipc.NewServer(ipc.Config{AppName: "dcal-export-test", APIVersion: 1}, handler)
	require.NoError(t, srv.Listen())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.Serve(ctx) }()
	t.Cleanup(func() { _ = srv.Close() })

	t.Setenv("DANKCAL_SOCKET", srv.SocketPath())

	result, err := remindersCall("calendars.export", map[string]any{"calendarId": "cal-1"})
	require.NoError(t, err)

	ics, ok := result.(string)
	require.True(t, ok, "expected string result, got %T", result)
	assert.Equal(t, "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n", ics)
}
