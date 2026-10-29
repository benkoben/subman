package subscription

import (
	"errors"
	"fmt"
	"subman/internal/payment"
	"subman/internal/plan"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/google/uuid"
)

var (
	invalidPaymentMeansErr error = errors.New("invalid payment means")
)

type Subscription struct {
	id        uuid.UUID
	userId    uuid.UUID
	means     payment.Means
	plan      plan.Plan
	trial     *trail
	reduction *Reduction

	startDate   time.Time
	cancelledAt *time.Time
	active      bool
}

type Option func(*Subscription)

func NewSubscription(userId uuid.UUID, subTo plan.Plan, means payment.Means, opts ...Option) (*Subscription, error) {
	if !means.IsActive() {
		return nil, invalidPaymentMeansErr
	}

	now := time.Now()
	s := &Subscription{
		id:        uuid.New(),
		userId:    userId,
		means:     means,
		plan:      subTo,
		startDate: now,
		active:    true,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

func WithReduction(reduction *Reduction) Option {
	return func(s *Subscription) {
		s.reduction = reduction
	}
}

// WithStartDate overrides default StartDate. Primary purpose is testing
func WithStartDate(startDate time.Time) Option {
	return func(s *Subscription) {
		s.startDate = startDate
	}
}

// calculateCostFromStart calculates how much a user has payed for a subscription
// based on the StartDate, CancelledAt, plan pricing and plan billing frequency.
func (s Subscription) calculateCostFromStart() (*money.Money, error) {
	var multiplier int64

	var stopDate time.Time
	if s.cancelledAt != nil {
		stopDate = *s.cancelledAt
	} else {
		stopDate = time.Now()
	}
	switch s.plan.BillingFrequency {
	case plan.Monthly:
		multiplier = int64(monthDifference(s.startDate, stopDate))
	case plan.Annually:
		multiplier = int64(yearDifference(s.startDate, stopDate))
	default:
		return nil, fmt.Errorf("unknown plan billing frequency")
	}

	return money.New(
		s.plan.Price.Amount()*multiplier, money.SEK,
	), nil
}

// Cancel deactivates a subscripton. Returns true if subscription.active toggled to false
// if nothing changes false is returned.
func (s *Subscription) Cancel() bool {
	if s.active {

		s.active = false
		now := time.Now()
		s.cancelledAt = &now
		return true
	}

	return false
}

// cancelWithTimestamp is a private function used to set a custom cancelledAt timestamp.
// primarily used for unit testing. No-op if subscription is not active.
func (s *Subscription) cancelWithTimestamp(ts time.Time) {
	if s.active {
		s.active = false
		s.cancelledAt = &ts
	}
}
