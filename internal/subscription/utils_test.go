package subscription

import (
	"testing"
	"time"

	"github.com/Rhymond/go-money"

	"subman/internal/plan"
)

// sek builds an in-package SEK amount for test fixtures.
func sek(amount int64) *money.Money {
	return money.New(amount, money.SEK)
}

func Test_stepPeriod(t *testing.T) {
	tests := []struct {
		name      string
		from      time.Time
		frequency plan.BillingFrequency
		want      time.Time
		wantErr   bool
	}{
		{
			name:      "mid month monthly",
			from:      time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC),
			frequency: plan.Monthly,
			want:      time.Date(2026, time.February, 15, 12, 0, 0, 0, time.UTC),
		},
		{
			name:      "month end normalizes through the shorter month",
			from:      time.Date(2026, time.January, 31, 12, 0, 0, 0, time.UTC),
			frequency: plan.Monthly,
			want:      time.Date(2026, time.March, 3, 12, 0, 0, 0, time.UTC),
		},
		{
			name:      "february end steps to the same day in march",
			from:      time.Date(2026, time.February, 28, 12, 0, 0, 0, time.UTC),
			frequency: plan.Monthly,
			want:      time.Date(2026, time.March, 28, 12, 0, 0, 0, time.UTC),
		},
		{
			name:      "december steps across the year boundary",
			from:      time.Date(2026, time.December, 15, 12, 0, 0, 0, time.UTC),
			frequency: plan.Monthly,
			want:      time.Date(2027, time.January, 15, 12, 0, 0, 0, time.UTC),
		},
		{
			name:      "annual keeps month and day",
			from:      time.Date(2025, time.January, 15, 12, 0, 0, 0, time.UTC),
			frequency: plan.Annually,
			want:      time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC),
		},
		{
			name:      "leap day annual normalizes to march 1",
			from:      time.Date(2024, time.February, 29, 12, 0, 0, 0, time.UTC),
			frequency: plan.Annually,
			want:      time.Date(2025, time.March, 1, 12, 0, 0, 0, time.UTC),
		},
		{
			name:      "unknown billing frequency",
			from:      time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC),
			frequency: plan.BillingFrequency("weekly"),
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stepPeriod(tt.from, tt.frequency)
			if (err != nil) != tt.wantErr {
				t.Fatalf("stepPeriod() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !got.Equal(tt.want) {
				t.Errorf("stepPeriod() = %s, want %s", got, tt.want)
			}
		})
	}
}

func Test_amountAt(t *testing.T) {
	intervals := []PriceInterval{
		{from: time.Date(2026, time.January, 17, 12, 0, 0, 0, time.UTC), amount: *sek(30)},
		{from: time.Date(2026, time.March, 17, 12, 0, 0, 0, time.UTC), amount: *sek(45)},
		{from: time.Date(2026, time.May, 17, 12, 0, 0, 0, time.UTC), amount: *sek(100)},
	}

	tests := []struct {
		name       string
		intervals  []PriceInterval
		at         time.Time
		wantAmount int64
	}{
		{
			name:       "exactly at the first interval From",
			intervals:  intervals,
			at:         time.Date(2026, time.January, 17, 12, 0, 0, 0, time.UTC),
			wantAmount: 30,
		},
		{
			name:       "between first and second uses the first amount",
			intervals:  intervals,
			at:         time.Date(2026, time.February, 17, 12, 0, 0, 0, time.UTC),
			wantAmount: 30,
		},
		{
			name:       "exactly at the second interval From uses the new amount",
			intervals:  intervals,
			at:         time.Date(2026, time.March, 17, 12, 0, 0, 0, time.UTC),
			wantAmount: 45,
		},
		{
			name:       "between second and third uses the second amount",
			intervals:  intervals,
			at:         time.Date(2026, time.April, 17, 12, 0, 0, 0, time.UTC),
			wantAmount: 45,
		},
		{
			name:       "after the last interval uses the last amount",
			intervals:  intervals,
			at:         time.Date(2027, time.June, 17, 12, 0, 0, 0, time.UTC),
			wantAmount: 100,
		},
		{
			name: "single interval applies forever",
			intervals: []PriceInterval{
				{from: time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC), amount: *sek(300)},
			},
			at:         time.Date(2030, time.January, 15, 12, 0, 0, 0, time.UTC),
			wantAmount: 300,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := amountAt(tt.intervals, tt.at)
			if got.Amount() != tt.wantAmount {
				t.Errorf("amountAt() amount = %d, want %d", got.Amount(), tt.wantAmount)
			}
			if got.Currency().Code != money.SEK {
				t.Errorf("amountAt() currency = %q, want %q", got.Currency().Code, money.SEK)
			}
		})
	}
}

func Test_sortIntervals(t *testing.T) {
	tests := []struct {
		name          string
		inFromMonths  []time.Month
		wantFromOrder []time.Month
	}{
		{
			name:          "already sorted stays sorted",
			inFromMonths:  []time.Month{time.January, time.March, time.May},
			wantFromOrder: []time.Month{time.January, time.March, time.May},
		},
		{
			name:          "out of order entries are reordered",
			inFromMonths:  []time.Month{time.May, time.January, time.March},
			wantFromOrder: []time.Month{time.January, time.March, time.May},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intervals := make([]PriceInterval, 0, len(tt.inFromMonths))
			for i, month := range tt.inFromMonths {
				intervals = append(intervals, PriceInterval{
					from:   time.Date(2026, month, 17, 12, 0, 0, 0, time.UTC),
					amount: *sek(int64(30 + 10*i)),
				})
			}

			sortIntervals(intervals)

			for i, wantMonth := range tt.wantFromOrder {
				_, gotMonth, _ := intervals[i].From().Date()
				if gotMonth != wantMonth {
					t.Fatalf("intervals[%d].From() month = %v, want %v", i, gotMonth, wantMonth)
				}
			}
		})
	}
}
