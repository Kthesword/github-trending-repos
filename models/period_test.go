package models

import (
	"testing"
	"time"
)

func TestParsePeriod(t *testing.T) {
	tests := []struct {
		in      string
		want    Period
		wantErr bool
	}{
		{in: "day", want: PeriodDay},
		{in: "WEEK", want: PeriodWeek},
		{in: "  month ", want: PeriodMonth},
		{in: "Year", want: PeriodYear},
		{in: "", wantErr: true},
		{in: "weeks", wantErr: true},
		{in: "decade", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParsePeriod(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParsePeriod(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParsePeriod(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestPeriodSince(t *testing.T) {
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		period Period
		want   time.Time
	}{
		{PeriodDay, time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)},
		{PeriodWeek, time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)},
		{PeriodMonth, time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)},
		{PeriodYear, time.Date(2025, 11, 1, 10, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(string(tt.period), func(t *testing.T) {
			if got := tt.period.Since(now); !got.Equal(tt.want) {
				t.Errorf("%s.Since(%v) = %v, want %v", tt.period, now, got, tt.want)
			}
		})
	}
}

func TestPeriodsListsEveryPeriod(t *testing.T) {
	got := Periods()
	want := []string{"day", "week", "month", "year"}
	if len(got) != len(want) {
		t.Fatalf("Periods() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Periods()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
