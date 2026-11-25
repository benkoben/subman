package subscription

import (
	"fmt"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/google/uuid"

	"subman/internal/plan"
)

// Phase is the derived lifecycle phase of a Subscription. It is never
// stored: PhaseAt reconstructs it from the subscription's dates.
type Phase string

const (
	PhaseTrialing  Phase = "trialing"
	PhaseActive    Phase = "active"
	PhaseCancelled Phase = "cancelled"
)

// PriceInterval is an entry in a Subscription's pricing history: a billing
// amount in SEK effective from a given date. The ordered list of intervals
// replaces a single flat price; the first paid interval is the Intro Offer.
type PriceInterval struct {
	from   time.Time
	amount money.Money
}

func (pi PriceInterval) From() time.Time {
	return pi.from
}

func (pi PriceInterval) Amount() money.Money {
	return pi.amount
}

// Subscription is a user's record of an ongoing paid relationship to a
// plan. One continuous record: its phase is derived from its dates, and
// cancellation is terminal — resubscribing creates a new Subscription.
type Subscription struct {
	id              uuid.UUID
	userID          uuid.UUID
	plan            plan.Plan
	paymentMethodID *uuid.UUID
	startDate       time.Time
	trialEnd        *time.Time
	cancelledAt     *time.Time
	intervals       []PriceInterval

	// trialDays is a construction-only carrier for the WithTrial option,
	// always nil once NewSubscription has returned.
	trialDays *int
}

type Option func(*Subscription)

// NewSubscription validates and constructs a Subscription. initialAmount
// becomes the first Price Interval (the Intro Offer), effective at the
// first charge date: the trial end when a trial was given, else the start
// date. The trial itself is free.
func NewSubscription(userID uuid.UUID, p plan.Plan, initialAmount int64, opts ...Option) (*Subscription, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("subscription user id cannot be zero")
	}
	if initialAmount <= 0 {
		return nil, fmt.Errorf("subscription initial amount must be positive, got %d", initialAmount)
	}

	s := &Subscription{
		id:        uuid.New(),
		userID:    userID,
		plan:      p,
		startDate: time.Now(),
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.paymentMethodID != nil && *s.paymentMethodID == uuid.Nil {
		return nil, fmt.Errorf("subscription payment method id cannot be zero")
	}
	if s.trialDays != nil {
		if *s.trialDays <= 0 {
			return nil, fmt.Errorf("subscription trial length must be positive, got %d days", *s.trialDays)
		}
		trialEnd := s.startDate.AddDate(0, 0, *s.trialDays)
		s.trialEnd = &trialEnd
	}
	s.trialDays = nil

	s.intervals = []PriceInterval{{
		from:   s.firstChargeDate(),
		amount: *money.New(initialAmount, money.SEK),
	}}

	return s, nil
}

// WithTrial sets a free opening trial of the given length in days. The
// trial end date is derived from the subscription's start date.
func WithTrial(days int) Option {
	return func(s *Subscription) {
		s.trialDays = &days
	}
}

// WithPaymentMethodID references a wallet entry by ID. Payment methods are
// optional metadata for bookkeeping; omitting the option means no payment
// method is tracked.
func WithPaymentMethodID(id uuid.UUID) Option {
	return func(s *Subscription) {
		s.paymentMethodID = &id
	}
}

// WithStartDate overrides the default start date (testing seam).
func WithStartDate(startDate time.Time) Option {
	return func(s *Subscription) {
		s.startDate = startDate
	}
}

func (s *Subscription) ID() uuid.UUID {
	return s.id
}

func (s *Subscription) UserID() uuid.UUID {
	return s.userID
}

func (s *Subscription) Plan() plan.Plan {
	return s.plan
}

func (s *Subscription) PaymentMethodID() *uuid.UUID {
	return s.paymentMethodID
}

func (s *Subscription) StartDate() time.Time {
	return s.startDate
}

func (s *Subscription) TrialEnd() *time.Time {
	return s.trialEnd
}

func (s *Subscription) CancelledAt() *time.Time {
	return s.cancelledAt
}

// Intervals returns a copy of the pricing history, sorted by From
// ascending. Mutating the returned slice does not affect the Subscription.
func (s *Subscription) Intervals() []PriceInterval {
	out := make([]PriceInterval, len(s.intervals))
	copy(out, s.intervals)
	return out
}

// PhaseAt derives the subscription's phase at the given moment. Cancellation
// is terminal and wins even during a trial; otherwise the phase is Trialing
// until the trial end, then Active.
func (s *Subscription) PhaseAt(now time.Time) Phase {
	if s.cancelledAt != nil {
		return PhaseCancelled
	}
	if s.trialEnd != nil && now.Before(*s.trialEnd) {
		return PhaseTrialing
	}
	return PhaseActive
}

// Phase is PhaseAt evaluated at the current moment.
func (s *Subscription) Phase() Phase {
	return s.PhaseAt(time.Now())
}

// Cancel records the user's in-app statement that the Subscription is no
// longer active spend. It is terminal: once cancelled, Cancel returns
// false without effect.
func (s *Subscription) Cancel() bool {
	return s.CancelAt(time.Now())
}

// CancelAt is Cancel with an explicit timestamp (testing seam). It has no
// effect and returns false when the subscription is already cancelled or
// when t is before the start date.
func (s *Subscription) CancelAt(t time.Time) bool {
	if s.cancelledAt != nil || t.Before(s.startDate) {
		return false
	}
	s.cancelledAt = &t
	return true
}

// AddPriceInterval appends an entry to the pricing history. The history is
// user-maintained: retroactive entries and corrections (a new entry at a
// new From) are allowed. It errors on a non-positive amount, a From before
// the first charge date, or a duplicate From. The history stays sorted by
// From ascending.
func (s *Subscription) AddPriceInterval(from time.Time, amount int64) error {
	if amount <= 0 {
		return fmt.Errorf("price interval amount must be positive, got %d", amount)
	}
	anchor := s.firstChargeDate()
	if from.Before(anchor) {
		return fmt.Errorf("price interval from %s is before the first charge date %s", from, anchor)
	}
	for _, iv := range s.intervals {
		if iv.From().Equal(from) {
			return fmt.Errorf("price interval already exists at %s", from)
		}
	}

	s.intervals = append(s.intervals, PriceInterval{
		from:   from,
		amount: *money.New(amount, money.SEK),
	})
	sortIntervals(s.intervals)

	return nil
}

// TotalPaidAt reconstructs the sum of charges incurred from the first
// charge until the cancellation, if cancelled, else the given now. Charges
// are counted in started billing periods — vendors bill upfront at each
// period start: at the first charge date, then every calendar month or
// year after it, each billed at the amount of the interval effective then.
func (s *Subscription) TotalPaidAt(now time.Time) (*money.Money, error) {
	anchor := s.firstChargeDate()

	stop := now
	if s.cancelledAt != nil {
		stop = *s.cancelledAt
	}
	if stop.Before(anchor) {
		return money.New(0, money.SEK), nil
	}

	total := money.New(0, money.SEK)
	for charge := anchor; !charge.After(stop); {
		next, err := stepPeriod(charge, s.plan.BillingFrequency)
		if err != nil {
			return nil, err
		}
		sum, err := total.Add(amountAt(s.intervals, charge))
		if err != nil {
			return nil, err
		}
		total = sum
		charge = next
	}

	return total, nil
}

// TotalPaid is TotalPaidAt evaluated at the current moment.
func (s *Subscription) TotalPaid() (*money.Money, error) {
	return s.TotalPaidAt(time.Now())
}

// firstChargeDate is the anchor of all billing: the trial end when the
// subscription was born with a trial, else the start date. The trial is
// free, so the first charge lands exactly at the anchor.
func (s *Subscription) firstChargeDate() time.Time {
	if s.trialEnd != nil {
		return *s.trialEnd
	}
	return s.startDate
}
