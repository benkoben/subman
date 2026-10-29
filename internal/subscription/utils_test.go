package subscription

import (
	"testing"
	"time"
)

func Test_isLeapYear(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		year int
		want bool
	}{
		{
			name: "leap year 2020",
			year: 2020,
			want: true,
		},
		{
			name: "non leap year 2026",
			year: 2026,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLeapYear(tt.year)
			if !got == tt.want {
				t.Errorf("isLeapYear() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_newMonthInDaysMap(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		year      int
		want      monthInDaysMap
		testMonth int
	}{
		{
			name: "leap year",
			year: 2020,
			want: monthInDaysMap{
				time.Month(1):  31,
				time.Month(2):  29,
				time.Month(3):  31,
				time.Month(4):  30,
				time.Month(5):  31,
				time.Month(6):  30,
				time.Month(7):  31,
				time.Month(8):  31,
				time.Month(9):  30,
				time.Month(10): 31,
				time.Month(11): 30,
				time.Month(12): 31,
			},
			testMonth: 2,
		},
		{
			name: "non leap year",
			year: 2026,
			want: monthInDaysMap{
				time.Month(1):  31,
				time.Month(2):  28,
				time.Month(3):  31,
				time.Month(4):  30,
				time.Month(5):  31,
				time.Month(6):  30,
				time.Month(7):  31,
				time.Month(8):  31,
				time.Month(9):  30,
				time.Month(10): 31,
				time.Month(11): 30,
				time.Month(12): 31,
			},
			testMonth: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newMonthInDaysMap(tt.year)
			// TODO: update the condition below to compare got with tt.want.
			if got[time.Month(tt.testMonth)] != tt.want[time.Month(tt.testMonth)] {
				t.Errorf("newMonthInDaysMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_monthInDaysMap_daysInRange(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		year int
		// Named input parameters for target function.
		start int
		stop  int
		want  int
	}{
		{
			name:  "january to march",
			year:  2026,
			start: 1,
			stop:  3,
			want:  90,
		},
		{
			name:  "whole year",
			year:  2026,
			start: 1,
			stop:  12,
			want:  365,
		},
		{
			name:  "whole leap year",
			year:  2020,
			start: 1,
			stop:  12,
			want:  366,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMonthInDaysMap(tt.year)
			got := m.daysInRange(tt.start, tt.stop)
			// TODO: update the condition below to compare got with tt.want.
			if got != tt.want {
				t.Errorf("daysInRange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_monthDifference(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		past  time.Time
		today time.Time
		want  int
	}{
		{
			name:  "same day",
			past:  time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC),
			today: time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC),
			want:  0,
		},
		{
			name:  "one complete month",
			past:  time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC),
			today: time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC),
			want:  1,
		},
		{
			name:  "day not reached yet",
			past:  time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC),
			today: time.Date(2026, time.February, 14, 0, 0, 0, 0, time.UTC),
			want:  0,
		},
		{
			name:  "day passed in next month",
			past:  time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC),
			today: time.Date(2026, time.February, 16, 0, 0, 0, 0, time.UTC),
			want:  1,
		},
		{
			name:  "year boundary completed month",
			past:  time.Date(2025, time.December, 15, 0, 0, 0, 0, time.UTC),
			today: time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC),
			want:  1,
		},
		{
			name:  "year boundary day not reached",
			past:  time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC),
			today: time.Date(2026, time.January, 30, 0, 0, 0, 0, time.UTC),
			want:  0,
		},
		{
			name:  "year boundary day reached",
			past:  time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC),
			today: time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC),
			want:  1,
		},
		{
			name:  "multiple years",
			past:  time.Date(2023, time.March, 10, 0, 0, 0, 0, time.UTC),
			today: time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC),
			want:  39,
		},
		{
			name:  "multiple years with incomplete month",
			past:  time.Date(2023, time.March, 20, 0, 0, 0, 0, time.UTC),
			today: time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC),
			want:  38,
		},
		{
			name:  "exactly one year",
			past:  time.Date(2025, time.June, 1, 0, 0, 0, 0, time.UTC),
			today: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
			want:  12,
		},
		{
			name:  "same month earlier day",
			past:  time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC),
			today: time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC),
			want:  -1,
		},
		{
			name:  "leap year February",
			past:  time.Date(2024, time.January, 29, 0, 0, 0, 0, time.UTC),
			today: time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC),
			want:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := monthDifference(tt.past, tt.today)
			// TODO: update the condition below to compare got with tt.want.
			if got != tt.want {
				t.Errorf("monthDifference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_yearDifference(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		past  time.Time
		today time.Time
		want  int
	}{
		{
			name:  "1 year difference",
			past:  time.Date(2025, time.January, 13, 0, 0, 0, 0, time.UTC),
			today: time.Date(2026, time.January, 13, 0, 0, 0, 0, time.UTC),
			want:  1,
		},
		{
			name:  "2025 and 2026 but no complete month yet",
			past:  time.Date(2025, time.January, 13, 0, 0, 0, 0, time.UTC),
			today: time.Date(2026, time.January, 12, 0, 0, 0, 0, time.UTC),
			want:  0,
		},
		{
			name:  "2025 and 2026 but no complete year yet",
			past:  time.Date(2025, time.February, 13, 0, 0, 0, 0, time.UTC),
			today: time.Date(2026, time.January, 13, 0, 0, 0, 0, time.UTC),
			want:  0,
		},
		{
			name:  "2020 and 2026 but no complete year yet",
			past:  time.Date(2020, time.February, 13, 0, 0, 0, 0, time.UTC),
			today: time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC),
			want:  6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := yearDifference(tt.past, tt.today)
			// TODO: update the condition below to compare got with tt.want.
			if got != tt.want {
				t.Errorf("yearDifference() = %v, want %v", got, tt.want)
			}
		})
	}
}
