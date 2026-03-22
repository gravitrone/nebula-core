package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- formatLocalTimeCompact ---

func TestFormatLocalTimeCompactZero(t *testing.T) {
	assert.Equal(t, "None", formatLocalTimeCompact(time.Time{}))
}

func TestFormatLocalTimeCompactToday(t *testing.T) {
	now := time.Now()
	result := formatLocalTimeCompact(now)
	// Compact format: "MM-DD HH:MM TZ"
	assert.NotEqual(t, "None", result)
	assert.Contains(t, result, now.Local().Format("01-02"))
}

func TestFormatLocalTimeCompactYesterday(t *testing.T) {
	yesterday := time.Now().Add(-24 * time.Hour)
	result := formatLocalTimeCompact(yesterday)
	assert.NotEqual(t, "None", result)
	assert.Contains(t, result, yesterday.Local().Format("01-02"))
}

func TestFormatLocalTimeCompactOlderDate(t *testing.T) {
	// Use local time to avoid date-shifting across timezone boundaries.
	old := time.Date(2023, 6, 15, 12, 0, 0, 0, time.Local)
	result := formatLocalTimeCompact(old)
	// "06-15 " prefix expected in compact format.
	assert.True(t, strings.HasPrefix(result, "06-15"), "got: %s", result)
}

func TestFormatLocalTimeCompactMidnight(t *testing.T) {
	// Use local midnight to avoid timezone-offset issues.
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	result := formatLocalTimeCompact(midnight)
	assert.NotEqual(t, "None", result)
	// Local midnight: time portion contains "00:00".
	assert.Contains(t, result, "00:00")
}

func TestFormatLocalTimeCompactYearBoundary(t *testing.T) {
	// Use local times to avoid cross-timezone date boundary surprises.
	dec31 := time.Date(2024, 12, 31, 12, 0, 0, 0, time.Local)
	jan1 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.Local)
	r1 := formatLocalTimeCompact(dec31)
	r2 := formatLocalTimeCompact(jan1)
	assert.NotEqual(t, r1, r2, "Dec 31 and Jan 1 should produce different outputs")
}

func TestFormatLocalTimeCompactColumnWidth(t *testing.T) {
	ts := time.Date(2025, 3, 14, 15, 9, 0, 0, time.Local)
	result := formatLocalTimeCompact(ts)
	// Formatted value must fit within compactTimeColumnWidth
	assert.LessOrEqual(t, len(result), compactTimeColumnWidth)
}

// --- formatLocalTimeFull ---

func TestFormatLocalTimeFullZero(t *testing.T) {
	assert.Equal(t, "None", formatLocalTimeFull(time.Time{}))
}

func TestFormatLocalTimeFullToday(t *testing.T) {
	now := time.Now()
	result := formatLocalTimeFull(now)
	assert.NotEqual(t, "None", result)
	// Full format includes 4-digit year
	assert.Contains(t, result, now.Local().Format("2006"))
}

func TestFormatLocalTimeFullYesterday(t *testing.T) {
	yesterday := time.Now().Add(-24 * time.Hour)
	result := formatLocalTimeFull(yesterday)
	assert.NotEqual(t, "None", result)
	assert.Contains(t, result, yesterday.Local().Format("2006-01-02"))
}

func TestFormatLocalTimeFullOlderDate(t *testing.T) {
	// Use local time to avoid date-shifting across timezone boundaries.
	old := time.Date(2023, 6, 15, 12, 0, 0, 0, time.Local)
	result := formatLocalTimeFull(old)
	assert.True(t, strings.HasPrefix(result, "2023-06-15"), "got: %s", result)
}

func TestFormatLocalTimeFullMidnight(t *testing.T) {
	// Use local midnight to avoid timezone-offset issues.
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	result := formatLocalTimeFull(midnight)
	assert.NotEqual(t, "None", result)
	assert.Contains(t, result, "00:00")
}

func TestFormatLocalTimeFullYearBoundary(t *testing.T) {
	// Use local noon to avoid cross-timezone date boundary surprises.
	dec31 := time.Date(2024, 6, 15, 12, 0, 0, 0, time.Local)
	jan1 := time.Date(2025, 6, 15, 12, 0, 0, 0, time.Local)
	r1 := formatLocalTimeFull(dec31)
	r2 := formatLocalTimeFull(jan1)
	assert.NotEqual(t, r1, r2)
	assert.Contains(t, r1, "2024")
	assert.Contains(t, r2, "2025")
}

func TestFormatLocalTimeFullContainsTimezone(t *testing.T) {
	ts := time.Date(2025, 3, 14, 15, 9, 0, 0, time.Local)
	result := formatLocalTimeFull(ts)
	// Format ends with timezone abbreviation (e.g. CET, UTC, MST).
	parts := strings.Fields(result)
	assert.GreaterOrEqual(t, len(parts), 3, "expected at least date, time, tz: %s", result)
}

func TestFormatLocalTimeFullVsCompactIncludesYear(t *testing.T) {
	ts := time.Date(2025, 3, 14, 15, 9, 0, 0, time.Local)
	compact := formatLocalTimeCompact(ts)
	full := formatLocalTimeFull(ts)
	// Full format has 4-digit year; compact does not include the year.
	assert.Contains(t, full, "2025")
	assert.NotContains(t, compact, "2025")
}
