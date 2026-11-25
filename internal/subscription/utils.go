package subscription

import (
	"fmt"
	"sort"
	"time"

	"github.com/Rhymond/go-money"

	"subman/internal/plan"
)

// stepPeriod returns the start of the next billing period after t: one
// calendar month for Monthly, one calendar year for Annually. It steps
// with time.Time.AddDate so month ends, month lengths and year boundaries
// stay calendar-true instead of being approximated by summed days.
func stepPeriod(t time.Time, frequency plan.BillingFrequency) (time.Time, error) {
	switch frequency {
	case plan.Monthly:
		return t.AddDate(0, 1, 0), nil
	case plan.Annually:
		return t.AddDate(1, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("unknown billing frequency %q", frequency)
	}
}

// amountAt returns the billing amount effective at t: the amount of the
// interval with the greatest From at or before t, so a charge exactly at
// From uses the new amount. intervals must be sorted by From ascending and
// t must not be before the first interval's From.
func amountAt(intervals []PriceInterval, t time.Time) *money.Money {
	effective := 0
	for i := range intervals {
		if intervals[i].From().After(t) {
			break
		}
		effective = i
	}
	return &intervals[effective].amount
}

// sortIntervals keeps a pricing history ordered by From ascending.
func sortIntervals(intervals []PriceInterval) {
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i].From().Before(intervals[j].From())
	})
}
