package models

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// Period is a named, relative time window used to decide how far back to
// look for trending repositories.
type Period string

// Supported periods.
const (
	PeriodDay   Period = "day"
	PeriodWeek  Period = "week"
	PeriodMonth Period = "month"
	PeriodYear  Period = "year"
)

// allPeriods lists every supported Period in ascending order of length.
var allPeriods = []Period{PeriodDay, PeriodWeek, PeriodMonth, PeriodYear}

// Periods returns the accepted period names, suitable for help text and
// error messages.
func Periods() []string {
	names := make([]string, len(allPeriods))
	for i, p := range allPeriods {
		names[i] = string(p)
	}
	return names
}

// ParsePeriod converts user input into a Period. Matching is
// case-insensitive and ignores surrounding whitespace.
func ParsePeriod(s string) (Period, error) {
	p := Period(strings.ToLower(strings.TrimSpace(s)))
	if slices.Contains(allPeriods, p) {
		return p, nil
	}
	return "", fmt.Errorf("invalid duration %q: must be one of %s",
		s, strings.Join(Periods(), ", "))
}

// Since returns the start of the window that ends at now.
//
// Calendar arithmetic (time.AddDate) is used for months and years so the
// window follows the calendar rather than a fixed number of hours.
func (p Period) Since(now time.Time) time.Time {
	switch p {
	case PeriodDay:
		return now.AddDate(0, 0, -1)
	case PeriodMonth:
		return now.AddDate(0, -1, 0)
	case PeriodYear:
		return now.AddDate(-1, 0, 0)
	default: // PeriodWeek and any unexpected value fall back to the default.
		return now.AddDate(0, 0, -7)
	}
}

// Description returns a human-friendly label such as "past 7 days".
func (p Period) Description() string {
	switch p {
	case PeriodDay:
		return "past 24 hours"
	case PeriodMonth:
		return "past month"
	case PeriodYear:
		return "past year"
	default:
		return "past 7 days"
	}
}
