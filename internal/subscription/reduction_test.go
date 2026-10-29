package subscription

import (
	"testing"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/google/go-cmp/cmp"
)

func TestNewReduction(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		count                int64
		reducedBillingAmount int64
		rType                ReductionType
		timeFunc             timeFn
		want                 Reduction
	}{
		{
			name:                 "3 months cost reduction",
			count:                3,
			reducedBillingAmount: 30,
			rType:                MonthlyReduction,
			timeFunc: func() time.Time {
				return time.Date(2026, time.January, 11, 14, 30, 0, 0, time.UTC)
			},
			want: Reduction{
				ActiveUntil:   time.Date(2026, time.April, 11, 14, 30, 0, 0, time.UTC),
				BillingAmount: money.New(30, "SEK"),
			},
		},
		{
			name:                 "3 months cost reduction leap year",
			count:                3,
			reducedBillingAmount: 30,
			rType:                MonthlyReduction,
			timeFunc: func() time.Time {
				return time.Date(2020, time.January, 11, 14, 30, 0, 0, time.UTC)
			},
			want: Reduction{
				ActiveUntil:   time.Date(2020, time.April, 11, 14, 30, 0, 0, time.UTC),
				BillingAmount: money.New(30, "SEK"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewReduction(tt.count, tt.reducedBillingAmount, tt.rType, tt.timeFunc)
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(Reduction{}, money.Money{})); diff != "" {
				t.Errorf("NewReduction() = %v, want %v", got, tt.want)
			}
		})
	}
}
