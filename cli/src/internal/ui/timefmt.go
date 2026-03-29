package ui

import "time"

const compactTimeColumnWidth = 18

// formatLocalTimeCompact handles format local time compact.
func formatLocalTimeCompact(ts time.Time) string {
	if ts.IsZero() {
		return "None"
	}
	return ts.Local().Format("01-02 15:04 MST")
}

// formatLocalTimeFull handles format local time full.
func formatLocalTimeFull(ts time.Time) string {
	if ts.IsZero() {
		return "None"
	}
	return ts.Local().Format("2006-01-02 15:04 MST")
}
