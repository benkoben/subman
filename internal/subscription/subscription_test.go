package subscription

import (
	"testing"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/google/uuid"

	"subman/internal/plan"
)

// mustPlan builds a valid plan for test fixtures.
func mustPlan(t *testing.T, frequency plan.BillingFrequency) plan.Plan {
	t.Helper()
	p, err := plan.NewPlan("Streaming Premium", "StreamCo", nil, frequency)
	if err != nil {
		t.Fatalf("plan.NewPlan() error = %v", err)
	}
	return p
}

// assertSEK checks that a computed amount is the expected SEK value.
func assertSEK(t *testing.T, label string, got money.Money, wantAmount int64) {
	t.Helper()
	if got.Amount() != wantAmount {
		t.Errorf("%s = %d SEK, want %d SEK", label, got.Amount(), wantAmount)
	}
	if got.Currency().Code != money.SEK {
		t.Errorf("%s currency = %q, want %q", label, got.Currency().Code, money.SEK)
	}
}

// date builds a fixed calendar date at mid-day UTC so that day and
// boundary comparisons stay unambiguous.
func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
}

func TestNewSubscription(t *testing.T) {
	startDate := date(2026, time.January, 10)
	paymentMethodID := uuid.New()

	tests := []struct {
		name    string
		userID  uuid.UUID
		plan    plan.Plan
		amount  int64
		opts    []Option
		wantErr bool
	}{
		{
			name:   "valid without options",
			userID: uuid.New(),
			plan:   mustPlan(t, plan.Monthly),
			amount: 30,
		},
		{
			name:   "valid with trial, start date and payment method",
			userID: uuid.New(),
			plan:   mustPlan(t, plan.Monthly),
			amount: 30,
			opts: []Option{
				WithTrial(7),
				WithStartDate(startDate),
				WithPaymentMethodID(paymentMethodID),
			},
		},
		{
			name:    "zero user id",
			userID:  uuid.Nil,
			plan:    mustPlan(t, plan.Monthly),
			amount:  30,
			wantErr: true,
		},
		{
			name:    "zero initial amount",
			userID:  uuid.New(),
			plan:    mustPlan(t, plan.Monthly),
			amount:  0,
			wantErr: true,
		},
		{
			name:    "negative initial amount",
			userID:  uuid.New(),
			plan:    mustPlan(t, plan.Monthly),
			amount:  -30,
			wantErr: true,
		},
		{
			name:    "zero trial length",
			userID:  uuid.New(),
			plan:    mustPlan(t, plan.Monthly),
			amount:  30,
			opts:    []Option{WithTrial(0)},
			wantErr: true,
		},
		{
			name:    "negative trial length",
			userID:  uuid.New(),
			plan:    mustPlan(t, plan.Monthly),
			amount:  30,
			opts:    []Option{WithTrial(-7)},
			wantErr: true,
		},
		{
			name:    "zero payment method id",
			userID:  uuid.New(),
			plan:    mustPlan(t, plan.Monthly),
			amount:  30,
			opts:    []Option{WithPaymentMethodID(uuid.Nil)},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewSubscription(tt.userID, tt.plan, tt.amount, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewSubscription() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if s.ID() == uuid.Nil {
				t.Errorf("ID() = zero uuid, want a generated id")
			}
			if s.UserID() != tt.userID {
				t.Errorf("UserID() = %v, want %v", s.UserID(), tt.userID)
			}
			if s.Plan() != tt.plan {
				t.Errorf("Plan() = %v, want %v", s.Plan(), tt.plan)
			}
			if s.PaymentMethodID() != nil && *s.PaymentMethodID() == uuid.Nil {
				t.Errorf("PaymentMethodID() = zero uuid")
			}
			if s.CancelledAt() != nil {
				t.Errorf("CancelledAt() = %v, want nil", s.CancelledAt())
			}
			intervals := s.Intervals()
			if len(intervals) != 1 {
				t.Fatalf("Intervals() length = %d, want 1", len(intervals))
			}
			assertSEK(t, "Intervals()[0].Amount()", intervals[0].Amount(), tt.amount)
		})
	}
}

func TestNewSubscription_TrialEndDerivedFromStartDate(t *testing.T) {
	startDate := date(2026, time.January, 10)
	wantTrialEnd := date(2026, time.January, 17)

	orders := []struct {
		name string
		opts []Option
	}{
		{
			name: "start date before trial",
			opts: []Option{WithStartDate(startDate), WithTrial(7)},
		},
		{
			name: "trial before start date",
			opts: []Option{WithTrial(7), WithStartDate(startDate)},
		},
	}
	for _, tt := range orders {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewSubscription(uuid.New(), mustPlan(t, plan.Monthly), 30, tt.opts...)
			if err != nil {
				t.Fatalf("NewSubscription() error = %v", err)
			}
			if !s.StartDate().Equal(startDate) {
				t.Errorf("StartDate() = %s, want %s", s.StartDate(), startDate)
			}
			if s.TrialEnd() == nil || !s.TrialEnd().Equal(wantTrialEnd) {
				t.Errorf("TrialEnd() = %v, want %s", s.TrialEnd(), wantTrialEnd)
			}
			// The trial is free: the first interval starts at the trial end.
			if !s.Intervals()[0].From().Equal(wantTrialEnd) {
				t.Errorf("Intervals()[0].From() = %s, want %s", s.Intervals()[0].From(), wantTrialEnd)
			}
		})
	}
}

func TestNewSubscription_FirstIntervalAtStartDateWithoutTrial(t *testing.T) {
	startDate := date(2026, time.January, 15)

	s, err := NewSubscription(uuid.New(), mustPlan(t, plan.Monthly), 30, WithStartDate(startDate))
	if err != nil {
		t.Fatalf("NewSubscription() error = %v", err)
	}
	if s.TrialEnd() != nil {
		t.Errorf("TrialEnd() = %v, want nil", s.TrialEnd())
	}
	if !s.Intervals()[0].From().Equal(startDate) {
		t.Errorf("Intervals()[0].From() = %s, want %s", s.Intervals()[0].From(), startDate)
	}
}

func TestSubscription_PhaseAt(t *testing.T) {
	tests := []struct {
		name string
		s    *Subscription
		now  time.Time
		want Phase
	}{
		{
			name: "during the trial",
			s: mustSubscription(t, plan.Monthly, 30,
				WithStartDate(date(2026, time.January, 10)), WithTrial(7)),
			now:  date(2026, time.January, 12),
			want: PhaseTrialing,
		},
		{
			name: "at the trial end the subscription is active",
			s: mustSubscription(t, plan.Monthly, 30,
				WithStartDate(date(2026, time.January, 10)), WithTrial(7)),
			now:  date(2026, time.January, 17),
			want: PhaseActive,
		},
		{
			name: "active without a trial",
			s:    mustSubscription(t, plan.Monthly, 30, WithStartDate(date(2026, time.January, 15))),
			now:  date(2026, time.February, 20),
			want: PhaseActive,
		},
		{
			name: "cancellation wins during a trial",
			s: cancelledSubscription(t, plan.Monthly, 30,
				[]Option{WithStartDate(date(2026, time.January, 10)), WithTrial(7)},
				date(2026, time.January, 12)),
			now:  date(2026, time.January, 13),
			want: PhaseCancelled,
		},
		{
			name: "cancellation after being active",
			s: cancelledSubscription(t, plan.Monthly, 30,
				[]Option{WithStartDate(date(2026, time.January, 15))},
				date(2026, time.March, 20)),
			now:  date(2026, time.March, 21),
			want: PhaseCancelled,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.PhaseAt(tt.now); got != tt.want {
				t.Errorf("PhaseAt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSubscription_Phase(t *testing.T) {
	s := mustSubscription(t, plan.Monthly, 30, WithStartDate(time.Now()))
	if got := s.Phase(); got != PhaseActive {
		t.Errorf("Phase() = %q, want %q", got, PhaseActive)
	}

	s.Cancel()
	if got := s.Phase(); got != PhaseCancelled {
		t.Errorf("Phase() after Cancel() = %q, want %q", got, PhaseCancelled)
	}
}

func TestSubscription_TotalPaidAt(t *testing.T) {
	tests := []struct {
		name string
		s    func(t *testing.T) *Subscription
		now  time.Time
		want int64
	}{
		{
			name: "monthly 30 SEK started Jan 15 2026, evaluated Feb 20 2026",
			s: func(t *testing.T) *Subscription {
				return mustSubscription(t, plan.Monthly, 30, WithStartDate(date(2026, time.January, 15)))
			},
			now:  date(2026, time.February, 20),
			want: 60, // charges Jan 15 + Feb 15
		},
		{
			name: "monthly 30 SEK started Jun 15 2026, cancelled Sep 16 2026",
			s: func(t *testing.T) *Subscription {
				return cancelledSubscription(t, plan.Monthly, 30,
					[]Option{WithStartDate(date(2026, time.June, 15))},
					date(2026, time.September, 16))
			},
			now:  date(2026, time.October, 1),
			want: 120, // charges Jun 15, Jul 15, Aug 15, Sep 15 — started periods
		},
		{
			name: "annual 300 SEK started Jan 15 2025, evaluated Mar 1 2025",
			s: func(t *testing.T) *Subscription {
				return mustSubscription(t, plan.Annually, 300, WithStartDate(date(2025, time.January, 15)))
			},
			now:  date(2025, time.March, 1),
			want: 300, // one charge
		},
		{
			name: "trial 7 days from Jan 10 2026, evaluated Jan 20 2026",
			s: func(t *testing.T) *Subscription {
				return mustSubscription(t, plan.Monthly, 30,
					WithStartDate(date(2026, time.January, 10)), WithTrial(7))
			},
			now:  date(2026, time.January, 20),
			want: 30, // first charge at the anchor Jan 17
		},
		{
			name: "trial plus intro offer, new amount from May 17 2026, evaluated Jun 1 2026",
			s: func(t *testing.T) *Subscription {
				s := mustSubscription(t, plan.Monthly, 30,
					WithStartDate(date(2026, time.January, 10)), WithTrial(7))
				if err := s.AddPriceInterval(date(2026, time.May, 17), 100); err != nil {
					t.Fatalf("AddPriceInterval() error = %v", err)
				}
				return s
			},
			now:  date(2026, time.June, 1),
			want: 220, // 30 at Jan 17, Feb 17, Mar 17, Apr 17, then 100 at May 17
		},
		{
			name: "cancelled during the trial",
			s: func(t *testing.T) *Subscription {
				return cancelledSubscription(t, plan.Monthly, 30,
					[]Option{WithStartDate(date(2026, time.January, 10)), WithTrial(7)},
					date(2026, time.January, 12))
			},
			now:  date(2026, time.February, 1),
			want: 0,
		},
		{
			name: "evaluated before the first charge date",
			s: func(t *testing.T) *Subscription {
				return mustSubscription(t, plan.Monthly, 30, WithStartDate(date(2026, time.January, 15)))
			},
			now:  date(2026, time.January, 14),
			want: 0,
		},
		{
			name: "a charge exactly at the stop still counts",
			s: func(t *testing.T) *Subscription {
				return mustSubscription(t, plan.Monthly, 30, WithStartDate(date(2026, time.January, 15)))
			},
			now:  date(2026, time.February, 15),
			want: 60,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.s(t).TotalPaidAt(tt.now)
			if err != nil {
				t.Fatalf("TotalPaidAt() error = %v", err)
			}
			assertSEK(t, "TotalPaidAt()", *got, tt.want)
		})
	}
}

func TestSubscription_TotalPaid(t *testing.T) {
	t.Run("one charge since a start at the current moment", func(t *testing.T) {
		s := mustSubscription(t, plan.Monthly, 30, WithStartDate(time.Now()))
		got, err := s.TotalPaid()
		if err != nil {
			t.Fatalf("TotalPaid() error = %v", err)
		}
		assertSEK(t, "TotalPaid()", *got, 30)
	})

	t.Run("ongoing trial has paid nothing", func(t *testing.T) {
		s := mustSubscription(t, plan.Monthly, 30, WithStartDate(time.Now()), WithTrial(7))
		got, err := s.TotalPaid()
		if err != nil {
			t.Fatalf("TotalPaid() error = %v", err)
		}
		assertSEK(t, "TotalPaid()", *got, 0)
	})
}

func TestSubscription_Cancel(t *testing.T) {
	t.Run("first cancel succeeds and stamps the time", func(t *testing.T) {
		s := mustSubscription(t, plan.Monthly, 30, WithStartDate(date(2026, time.January, 15)))
		if !s.Cancel() {
			t.Fatal("Cancel() = false, want true")
		}
		if s.CancelledAt() == nil {
			t.Fatal("CancelledAt() = nil, want a timestamp")
		}
	})

	t.Run("second cancel is a no-op", func(t *testing.T) {
		s := mustSubscription(t, plan.Monthly, 30, WithStartDate(date(2026, time.January, 15)))
		s.Cancel()
		first := s.CancelledAt()
		if s.Cancel() {
			t.Fatal("second Cancel() = true, want false")
		}
		if !s.CancelledAt().Equal(*first) {
			t.Errorf("CancelledAt() changed on second Cancel(): %s, want %s", s.CancelledAt(), first)
		}
	})
}

func TestSubscription_CancelAt(t *testing.T) {
	t.Run("before the start date has no effect", func(t *testing.T) {
		s := mustSubscription(t, plan.Monthly, 30, WithStartDate(date(2026, time.January, 15)))
		if s.CancelAt(date(2026, time.January, 14)) {
			t.Fatal("CancelAt() before startDate = true, want false")
		}
		if s.CancelledAt() != nil {
			t.Errorf("CancelledAt() = %s, want nil", s.CancelledAt())
		}
		if got := s.PhaseAt(date(2026, time.February, 1)); got != PhaseActive {
			t.Errorf("PhaseAt() = %q, want %q", got, PhaseActive)
		}
	})

	t.Run("at or after the start date cancels with the given timestamp", func(t *testing.T) {
		s := mustSubscription(t, plan.Monthly, 30, WithStartDate(date(2026, time.January, 15)))
		at := date(2026, time.March, 20)
		if !s.CancelAt(at) {
			t.Fatal("CancelAt() = false, want true")
		}
		if !s.CancelledAt().Equal(at) {
			t.Errorf("CancelledAt() = %s, want %s", s.CancelledAt(), at)
		}
	})

	t.Run("already cancelled stays cancelled", func(t *testing.T) {
		s := mustSubscription(t, plan.Monthly, 30, WithStartDate(date(2026, time.January, 15)))
		s.CancelAt(date(2026, time.March, 20))
		if s.CancelAt(date(2026, time.April, 20)) {
			t.Fatal("CancelAt() on a cancelled subscription = true, want false")
		}
		if !s.CancelledAt().Equal(date(2026, time.March, 20)) {
			t.Errorf("CancelledAt() = %s, want the earlier timestamp", s.CancelledAt())
		}
	})
}

func TestSubscription_AddPriceInterval(t *testing.T) {
	t.Run("from before the first charge date", func(t *testing.T) {
		s := mustSubscription(t, plan.Monthly, 30, WithStartDate(date(2026, time.January, 15)))
		if err := s.AddPriceInterval(date(2026, time.January, 14), 45); err == nil {
			t.Fatal("AddPriceInterval() before the anchor = nil error, want an error")
		}
	})

	t.Run("duplicate from", func(t *testing.T) {
		s := mustSubscription(t, plan.Monthly, 30, WithStartDate(date(2026, time.January, 15)))
		if err := s.AddPriceInterval(date(2026, time.March, 15), 45); err != nil {
			t.Fatalf("first AddPriceInterval() error = %v", err)
		}
		if err := s.AddPriceInterval(date(2026, time.March, 15), 50); err == nil {
			t.Fatal("duplicate AddPriceInterval() = nil error, want an error")
		}
		if err := s.AddPriceInterval(date(2026, time.January, 15), 50); err == nil {
			t.Fatal("duplicate of the anchor interval = nil error, want an error")
		}
	})

	t.Run("zero and negative amounts", func(t *testing.T) {
		s := mustSubscription(t, plan.Monthly, 30, WithStartDate(date(2026, time.January, 15)))
		if err := s.AddPriceInterval(date(2026, time.March, 15), 0); err == nil {
			t.Fatal("AddPriceInterval() with amount 0 = nil error, want an error")
		}
		if err := s.AddPriceInterval(date(2026, time.March, 15), -45); err == nil {
			t.Fatal("AddPriceInterval() with a negative amount = nil error, want an error")
		}
	})

	t.Run("retroactive entry is accepted and the history stays sorted", func(t *testing.T) {
		s := mustSubscription(t, plan.Monthly, 30, WithStartDate(date(2026, time.January, 15)))
		// Added out of order on purpose.
		if err := s.AddPriceInterval(date(2026, time.May, 15), 100); err != nil {
			t.Fatalf("AddPriceInterval() error = %v", err)
		}
		if err := s.AddPriceInterval(date(2026, time.March, 15), 45); err != nil {
			t.Fatalf("retroactive AddPriceInterval() error = %v", err)
		}

		intervals := s.Intervals()
		wantFroms := []time.Time{
			date(2026, time.January, 15),
			date(2026, time.March, 15),
			date(2026, time.May, 15),
		}
		if len(intervals) != len(wantFroms) {
			t.Fatalf("Intervals() length = %d, want %d", len(intervals), len(wantFroms))
		}
		for i, wantFrom := range wantFroms {
			if !intervals[i].From().Equal(wantFrom) {
				t.Errorf("Intervals()[%d].From() = %s, want %s", i, intervals[i].From(), wantFrom)
			}
		}
		assertSEK(t, "Intervals()[2].Amount()", intervals[2].Amount(), 100)
	})
}

func TestSubscription_Intervals_ReturnsACopy(t *testing.T) {
	s := mustSubscription(t, plan.Monthly, 30, WithStartDate(date(2026, time.January, 15)))

	intervals := s.Intervals()
	intervals[0].from = date(2030, time.January, 1)
	intervals = append(intervals, PriceInterval{from: date(2031, time.January, 1)})

	fresh := s.Intervals()
	if len(fresh) != 1 {
		t.Fatalf("Intervals() length = %d, want 1", len(fresh))
	}
	if !fresh[0].From().Equal(date(2026, time.January, 15)) {
		t.Errorf("Intervals()[0].From() = %s, want the untouched anchor", fresh[0].From())
	}
}

// mustSubscription builds a valid subscription for test fixtures.
func mustSubscription(t *testing.T, frequency plan.BillingFrequency, initialAmount int64, opts ...Option) *Subscription {
	t.Helper()
	s, err := NewSubscription(uuid.New(), mustPlan(t, frequency), initialAmount, opts...)
	if err != nil {
		t.Fatalf("NewSubscription() error = %v", err)
	}
	return s
}

// cancelledSubscription builds a valid subscription and cancels it at the
// given timestamp.
func cancelledSubscription(t *testing.T, frequency plan.BillingFrequency, initialAmount int64, opts []Option, at time.Time) *Subscription {
	t.Helper()
	s := mustSubscription(t, frequency, initialAmount, opts...)
	if !s.CancelAt(at) {
		t.Fatalf("CancelAt(%s) = false, want true", at)
	}
	return s
}
