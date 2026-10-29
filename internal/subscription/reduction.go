package subscription

import (
	"time"

	"github.com/Rhymond/go-money"
)

// A Reduction can be added to a subsription in cases where a vendor
// offers a reduced price until a certain date. typically something like:
// 3 months for 30SEK then 100SEK after that.

type ReductionType string

const (
	MonthlyReduction ReductionType = "monthly"
	YearlyReduction  ReductionType = "yearly"
)

type Reduction struct {
	ActiveUntil   time.Time
	BillingAmount *money.Money
}

// timeFn is used for dependency inejcetion
type timeFn func() time.Time

func NewReduction(count, reducedBillingAmount int64, rType ReductionType, timeFunc timeFn) Reduction {
	var today time.Time
	var monthNow time.Month

	if timeFunc != nil {
		today = timeFunc()
		monthNow = today.Month()
	} else {
		today = time.Now()
		monthNow = today.Month()
	}

	monthsInYear := newMonthInDaysMap(today.Year())

	var sumDays int
	stop := int(monthNow) + int(count) - 1
	for i := int(monthNow); i <= stop; i++ {
		sumDays += monthsInYear[time.Month(i)]
	}

	day := time.Duration(time.Hour * 24)
	total := time.Duration(day * time.Duration(sumDays))
	until := today.Add(total)
	amount := money.New(reducedBillingAmount, "SEK")

	return Reduction{
		ActiveUntil:   until,
		BillingAmount: amount,
	}
}
