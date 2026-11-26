package api

import (
	"errors"
	"net/http"
	"time"

	"subman/internal/app"
	"subman/internal/payment"
	"subman/internal/repository"
	"subman/internal/subscription"
	"subman/internal/user"
)

func toCardResponse(c *payment.Card, now time.Time) cardResponse {
	year, month := c.Expiry()
	var removedAt *time.Time
	if c.RemovedAt() != nil {
		t := *c.RemovedAt()
		removedAt = &t
	}

	return cardResponse{
		ID:          c.ID().String(),
		Name:        c.Name(),
		CardType:    string(c.CardType()),
		ExpiryYear:  year,
		ExpiryMonth: int(month),
		RemovedAt:   removedAt,
		Expired:     c.ExpiredAt(now),
		Active:      c.IsActiveAt(now),
	}
}

func toSubscriptionResponse(s *subscription.Subscription, now time.Time) (subscriptionResponse, error) {
	var paymentMethodID *string
	if s.PaymentMethodID() != nil {
		id := s.PaymentMethodID().String()
		paymentMethodID = &id
	}

	intervals := make([]intervalResponse, 0)
	for _, iv := range s.Intervals() {
		amount := iv.Amount()
		intervals = append(intervals, intervalResponse{From: iv.From(), Amount: amount.Amount()})
	}

	totalPaid, err := s.TotalPaidAt(now)
	if err != nil {
		return subscriptionResponse{}, err
	}

	p := s.Plan()
	return subscriptionResponse{
		ID: s.ID().String(),
		Plan: planResponse{
			Name:             p.Name,
			Vendor:           p.Vendor,
			Description:      p.Description,
			BillingFrequency: string(p.BillingFrequency),
		},
		PaymentMethodID: paymentMethodID,
		StartDate:       s.StartDate(),
		TrialEnd:        s.TrialEnd(),
		CancelledAt:     s.CancelledAt(),
		Intervals:       intervals,
		Phase:           string(s.PhaseAt(now)),
		TotalPaid: moneyResponse{
			Amount:   totalPaid.Amount(),
			Currency: totalPaid.Currency().Code,
		},
	}, nil
}

func toUserResponse(u user.User) userResponse {
	return userResponse{
		ID:    u.ID().String(),
		Name:  u.Name(),
		Email: u.Email(),
	}
}

func writeAppError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, app.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, repository.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

type profileResponse struct {
	User          userResponse           `json:"user"`
	Cards         []cardResponse         `json:"cards"`
	Subscriptions []subscriptionResponse `json:"subscriptions"`
}

type userResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type subscriptionResponse struct {
	ID              string             `json:"id"`
	Plan            planResponse       `json:"plan"`
	PaymentMethodID *string            `json:"paymentMethodID"`
	StartDate       time.Time          `json:"startDate"`
	TrialEnd        *time.Time         `json:"trialEnd"`
	CancelledAt     *time.Time         `json:"cancelledAt"`
	Intervals       []intervalResponse `json:"intervals"`
	Phase           string             `json:"phase"`
	TotalPaid       moneyResponse      `json:"totalPaid"`
}

type moneyResponse struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

type planResponse struct {
	Name             string  `json:"name"`
	Vendor           string  `json:"vendor"`
	Description      *string `json:"description"`
	BillingFrequency string  `json:"billingFrequency"`
}

type intervalResponse struct {
	From   time.Time `json:"from"`
	Amount int64     `json:"amount"`
}

type cardResponse struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	CardType    string     `json:"cardType"`
	ExpiryYear  int        `json:"expiryYear"`
	ExpiryMonth int        `json:"expiryMonth"`
	RemovedAt   *time.Time `json:"removedAt"`
	Expired     bool       `json:"expired"`
	Active      bool       `json:"active"`
}
