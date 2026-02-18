package ui

import "time"

const compactTimeColumnWidth = 15

func formatLocalTimeCompact(ts time.Time) string {
	if ts.IsZero() {
		return "None"
	}
	return ts.Local().Format("01-02 15:04 MST")
}

func formatLocalTimeFull(ts time.Time) string {
	if ts.IsZero() {
		return "None"
	}
	return ts.Local().Format("2006-01-02 15:04 MST")
}
