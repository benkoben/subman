package subscription

import (
	"subman/internal/payment"
	"subman/internal/plan"
	"testing"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/google/uuid"
)

func TestSubscription_Cancel(t *testing.T) {
	tests := []struct {
		name      string
		userId    uuid.UUID
		subTo     plan.Plan
		means     payment.Means
		reduction *Reduction
		want      bool
	}{
		{
			name:   "Active subscription sucessfully cancelled",
			userId: uuid.New(),
			subTo: plan.NewPlan(
				"dummy",
				"mooooooovies",
				nil,
				80,
				plan.Monthly,
			),
			means:     payment.NewCard("muchMoneys", payment.Debid, time.Now().Add(time.Hour*24*365)),
			reduction: nil,
			want:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewSubscription(tt.userId, tt.subTo, tt.means, WithReduction(tt.reduction))
			if err != nil {
				t.Fatalf("could not construct receiver type: %v", err)
			}
			got := s.Cancel()
			// TODO: update the condition below to compare got with tt.want.
			if got != tt.want {
				t.Errorf("Cancel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateCostFromStart(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		constructor func() *Subscription
		want        *money.Money
		wantErr     error
	}{
		{
			name: "monthly billed twice",
			constructor: func() *Subscription {
				userId := uuid.New()
				subTo := plan.NewPlan("movies", "vendor", nil, 30, plan.Monthly)
				card := payment.NewCard("dummyCard", payment.Credit, time.Now().Add(time.Hour*24*365))
				s, _ := NewSubscription(userId, subTo, card, WithStartDate(time.Now().Add(-time.Hour*24*62))) // 2 months in the past start date
				return s
			},
			want:    money.New(60, money.SEK),
			wantErr: nil,
		},
		{
			name: "monthly billed 7 times",
			constructor: func() *Subscription {
				userId := uuid.New()
				subTo := plan.NewPlan("movies", "vendor", nil, 30, plan.Monthly)
				card := payment.NewCard("dummyCard", payment.Credit, time.Now().Add(time.Hour*24*365))
				s, _ := NewSubscription(userId, subTo, card, WithStartDate(time.Now().Add(-time.Hour*24*215))) // 7 months in the past start date
				return s
			},
			want:    money.New(210, money.SEK),
			wantErr: nil,
		},
		{
			name: "subscription cancelled now but was active for 3 months",
			constructor: func() *Subscription {
				userId := uuid.New()
				subTo := plan.NewPlan("movies", "vendor", nil, 30, plan.Monthly)
				card := payment.NewCard("dummyCard", payment.Credit, time.Now().Add(time.Hour*24*365))

				startDate := time.Now().Add(-time.Hour * 24 * 186)      // ~6months in the past
				cancelledAtDate := time.Now().Add(-time.Hour * 24 * 93) // ~3months in the past
				s, _ := NewSubscription(userId, subTo, card, WithStartDate(startDate))
				s.cancelWithTimestamp(cancelledAtDate)
				return s
			},
			want:    money.New(90, money.SEK), // 3 months active * 30SEK = 90SEK
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.constructor()
			got, gotErr := s.calculateCostFromStart()
			if gotErr == nil && tt.wantErr != nil {
				t.Fatal("expected error but none was returned")
			}

			// Likely not happening but protects against nil dereferences
			if got == nil && tt.want != nil {
				t.Errorf("calculateCostFromStart() returned nil, expected %v", *tt.want)
			}
			if *got != *tt.want {
				t.Errorf("calculateCostFromStart() = %v, want %v", got, tt.want)
			}
		})
	}
}
