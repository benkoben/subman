package subscription

import (
	"fmt"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"

	"subman/internal/plan"
)

// subscriptionDoc is the persisted shape of a Subscription. Its fields are
// unexported, so the package owns its own mapping: UUIDs are stored as
// strings, times as bson-native time.Time, amounts as the int64 SEK value
// only (SEK-only is ADR 0001, so the currency is never stored), the plan
// value object is stored inline, paymentMethodID/trialEnd/cancelledAt are
// nullable, and the pricing history is an array of {from, amount}.
type subscriptionDoc struct {
	ID              string         `bson:"_id"`
	UserID          string         `bson:"userID"`
	Plan            planDoc        `bson:"plan"`
	PaymentMethodID *string        `bson:"paymentMethodID,omitempty"`
	StartDate       time.Time      `bson:"startDate"`
	TrialEnd        *time.Time     `bson:"trialEnd,omitempty"`
	CancelledAt     *time.Time     `bson:"cancelledAt,omitempty"`
	Intervals       []intervalDoc  `bson:"intervals"`
}

// planDoc is the inline persistence of the plan value object owned by the
// Subscription.
type planDoc struct {
	Name             string  `bson:"name"`
	Vendor           string  `bson:"vendor"`
	Description      *string `bson:"description,omitempty"`
	BillingFrequency string  `bson:"billingFrequency"`
}

// intervalDoc is one entry of the pricing history.
type intervalDoc struct {
	From   time.Time `bson:"from"`
	Amount int64     `bson:"amount"`
}

// MarshalBSON encodes the Subscription as its persisted document. The
// value receiver lets the driver marshal both Subscription and
// *Subscription values.
func (s Subscription) MarshalBSON() ([]byte, error) {
	paymentMethodID := (*string)(nil)
	if s.paymentMethodID != nil {
		id := s.paymentMethodID.String()
		paymentMethodID = &id
	}

	intervals := make([]intervalDoc, len(s.intervals))
	for i, iv := range s.intervals {
		amount := iv.Amount()
		intervals[i] = intervalDoc{From: iv.From(), Amount: amount.Amount()}
	}

	return bson.Marshal(subscriptionDoc{
		ID:              s.id.String(),
		UserID:          s.userID.String(),
		Plan: planDoc{
			Name:             s.plan.Name,
			Vendor:           s.plan.Vendor,
			Description:      s.plan.Description,
			BillingFrequency: string(s.plan.BillingFrequency),
		},
		PaymentMethodID: paymentMethodID,
		StartDate:       s.startDate,
		TrialEnd:        s.trialEnd,
		CancelledAt:     s.cancelledAt,
		Intervals:       intervals,
	})
}

// UnmarshalBSON decodes a persisted document back into the Subscription.
func (s *Subscription) UnmarshalBSON(data []byte) error {
	var d subscriptionDoc
	if err := bson.Unmarshal(data, &d); err != nil {
		return err
	}

	id, err := uuid.Parse(d.ID)
	if err != nil {
		return fmt.Errorf("subscription id %q is not a UUID: %w", d.ID, err)
	}
	userID, err := uuid.Parse(d.UserID)
	if err != nil {
		return fmt.Errorf("subscription user id %q is not a UUID: %w", d.UserID, err)
	}

	var paymentMethodID *uuid.UUID
	if d.PaymentMethodID != nil {
		pmID, err := uuid.Parse(*d.PaymentMethodID)
		if err != nil {
			return fmt.Errorf("subscription payment method id %q is not a UUID: %w", *d.PaymentMethodID, err)
		}
		paymentMethodID = &pmID
	}

	intervals := make([]PriceInterval, len(d.Intervals))
	for i, iv := range d.Intervals {
		if iv.Amount <= 0 {
			return fmt.Errorf("price interval amount must be positive, got %d", iv.Amount)
		}
		intervals[i] = PriceInterval{
			from:   iv.From,
			amount: *money.New(iv.Amount, money.SEK),
		}
	}

	s.id = id
	s.userID = userID
	s.plan = plan.Plan{
		Name:             d.Plan.Name,
		Vendor:           d.Plan.Vendor,
		Description:      d.Plan.Description,
		BillingFrequency: plan.BillingFrequency(d.Plan.BillingFrequency),
	}
	s.paymentMethodID = paymentMethodID
	s.startDate = d.StartDate
	s.trialEnd = d.TrialEnd
	s.cancelledAt = d.CancelledAt
	s.intervals = intervals

	return nil
}
