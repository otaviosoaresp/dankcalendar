package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var exportOutput string

var exportCmd = &cobra.Command{
	Use:   "export <calendar>",
	Short: "Export a calendar as iCalendar text",
	Long:  "Export every event in a calendar to iCalendar text, written to stdout or -o. Find calendar ids with 'dcal ipc calendars.list'.",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		result, err := remindersCall("calendars.export", map[string]any{"calendarId": args[0]})
		if err != nil {
			return err
		}
		ics, ok := result.(string)
		if !ok {
			return fmt.Errorf("unexpected response from calendars.export")
		}
		if exportOutput == "" {
			fmt.Fprint(os.Stdout, ics)
			return nil
		}
		return os.WriteFile(exportOutput, []byte(ics), 0o644)
	},
}

func init() {
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Write to this file instead of stdout")
}
