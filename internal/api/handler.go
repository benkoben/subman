package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"subman/internal/app"
	"subman/internal/payment"
	"subman/internal/plan"
	"subman/internal/repository"
	"subman/internal/subscription"
	"subman/internal/user"
)

type options struct {
	clock func() time.Time
}

// Option configures NewHandler.
type Option func(*options)

// WithClock sets the source of the current time used to evaluate derived
// state in responses, such as a Subscription's phase and total paid. Each
// response reads the clock once.
func WithClock(clock func() time.Time) Option {
	return func(o *options) {
		o.clock = clock
	}
}

// NewHandler routes the HTTP API onto the given Service. Without options it
// evaluates derived state at time.Now.
func NewHandler(svc *app.Service, opts ...Option) http.Handler {
	cfg := options{clock: time.Now}
	for _, opt := range opts {
		opt(&cfg)
	}
	clock := cfg.clock

	mux := http.NewServeMux()
	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		var req userRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		u, err := svc.CreateUser(r.Context(), req.Name, req.Email)
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, toUserResponse(u))
	})
	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		users, err := svc.ListUsers(r.Context())
		if err != nil {
			writeAppError(w, err)
			return
		}

		out := make([]userResponse, 0, len(users))
		for _, u := range users {
			out = append(out, toUserResponse(u))
		}

		writeJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("POST /users/{userID}/cards", func(w http.ResponseWriter, r *http.Request) {
		userID, err := uuid.Parse(r.PathValue("userID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "userID must be a UUID")
			return
		}

		var req cardRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		c, err := svc.CreateCard(r.Context(), userID, req.Name, payment.CardType(req.CardType),
			req.ExpiryYear, time.Month(req.ExpiryMonth))
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, toCardResponse(c, clock()))
	})
	mux.HandleFunc("GET /users/{userID}/cards", func(w http.ResponseWriter, r *http.Request) {
		userID, err := uuid.Parse(r.PathValue("userID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "userID must be a UUID")
			return
		}

		cards, err := svc.ListCards(r.Context(), userID)
		if err != nil {
			writeAppError(w, err)
			return
		}

		now := clock()
		out := make([]cardResponse, 0, len(cards))
		for _, c := range cards {
			out = append(out, toCardResponse(c, now))
		}

		writeJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("POST /users/{userID}/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		userID, err := uuid.Parse(r.PathValue("userID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "userID must be a UUID")
			return
		}

		var req subscriptionRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.StartDate == nil {
			writeError(w, http.StatusBadRequest, "startDate is required")
			return
		}

		in := app.SubscriptionInput{
			PlanName:         req.Plan.Name,
			PlanVendor:       req.Plan.Vendor,
			PlanDescription:  req.Plan.Description,
			BillingFrequency: plan.BillingFrequency(req.Plan.BillingFrequency),
			InitialAmount:    req.InitialAmount,
			StartDate:        *req.StartDate,
			TrialDays:        req.TrialDays,
		}
		if req.PaymentMethodID != nil {
			paymentMethodID, err := uuid.Parse(*req.PaymentMethodID)
			if err != nil {
				writeError(w, http.StatusBadRequest, "paymentMethodID must be a UUID")
				return
			}
			in.PaymentMethodID = &paymentMethodID
		}

		s, err := svc.CreateSubscription(r.Context(), userID, in)
		if err != nil {
			writeAppError(w, err)
			return
		}

		out, err := toSubscriptionResponse(s, clock())
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, out)
	})
	mux.HandleFunc("GET /users/{userID}/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		userID, err := uuid.Parse(r.PathValue("userID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "userID must be a UUID")
			return
		}

		subs, err := svc.ListSubscriptions(r.Context(), userID)
		if err != nil {
			writeAppError(w, err)
			return
		}

		now := clock()
		out := make([]subscriptionResponse, 0, len(subs))
		for _, s := range subs {
			resp, err := toSubscriptionResponse(s, now)
			if err != nil {
				writeAppError(w, err)
				return
			}
			out = append(out, resp)
		}

		writeJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("POST /users/{userID}/subscriptions/{subscriptionID}/cancellation", func(w http.ResponseWriter, r *http.Request) {
		userID, err := uuid.Parse(r.PathValue("userID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "userID must be a UUID")
			return
		}
		subscriptionID, err := uuid.Parse(r.PathValue("subscriptionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "subscriptionID must be a UUID")
			return
		}

		var req subscriptionCancellationRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.CancelledAt == nil {
			writeError(w, http.StatusBadRequest, "cancelledAt is required")
			return
		}

		s, err := svc.CancelSubscription(r.Context(), userID, subscriptionID, *req.CancelledAt)
		if err != nil {
			writeAppError(w, err)
			return
		}

		out, err := toSubscriptionResponse(s, clock())
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("POST /users/{userID}/subscriptions/{subscriptionID}/price-intervals", func(w http.ResponseWriter, r *http.Request) {
		userID, err := uuid.Parse(r.PathValue("userID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "userID must be a UUID")
			return
		}
		subscriptionID, err := uuid.Parse(r.PathValue("subscriptionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "subscriptionID must be a UUID")
			return
		}

		var req priceIntervalRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.From == nil {
			writeError(w, http.StatusBadRequest, "from is required")
			return
		}

		s, err := svc.AddPriceInterval(r.Context(), userID, subscriptionID, *req.From, req.Amount)
		if err != nil {
			writeAppError(w, err)
			return
		}

		out, err := toSubscriptionResponse(s, clock())
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("GET /users/{userID}/profile", func(w http.ResponseWriter, r *http.Request) {
		userID, err := uuid.Parse(r.PathValue("userID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "userID must be a UUID")
			return
		}

		p, err := svc.Profile(r.Context(), userID)
		if err != nil {
			writeAppError(w, err)
			return
		}

		now := clock()
		cards := make([]cardResponse, 0, len(p.Cards))
		for _, c := range p.Cards {
			cards = append(cards, toCardResponse(c, now))
		}
		subs := make([]subscriptionResponse, 0, len(p.Subscriptions))
		for _, s := range p.Subscriptions {
			resp, err := toSubscriptionResponse(s, now)
			if err != nil {
				writeAppError(w, err)
				return
			}
			subs = append(subs, resp)
		}

		writeJSON(w, http.StatusOK, profileResponse{
			User:          toUserResponse(p.User),
			Cards:         cards,
			Subscriptions: subs,
		})
	})
	mux.HandleFunc("POST /users/{userID}/cards/{cardID}/removal", func(w http.ResponseWriter, r *http.Request) {
		userID, err := uuid.Parse(r.PathValue("userID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "userID must be a UUID")
			return
		}
		cardID, err := uuid.Parse(r.PathValue("cardID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "cardID must be a UUID")
			return
		}

		var req cardRemovalRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.RemovedAt == nil {
			writeError(w, http.StatusBadRequest, "removedAt is required")
			return
		}

		c, err := svc.RemoveCard(r.Context(), userID, cardID, *req.RemovedAt)
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, toCardResponse(c, clock()))
	})

	return mux
}

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

type userRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type userResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type cardRequest struct {
	Name        string `json:"name"`
	CardType    string `json:"cardType"`
	ExpiryYear  int    `json:"expiryYear"`
	ExpiryMonth int    `json:"expiryMonth"`
}

type cardRemovalRequest struct {
	RemovedAt *time.Time `json:"removedAt"`
}

type subscriptionCancellationRequest struct {
	CancelledAt *time.Time `json:"cancelledAt"`
}

type priceIntervalRequest struct {
	From   *time.Time `json:"from"`
	Amount int64      `json:"amount"`
}

type subscriptionRequest struct {
	Plan            planRequest `json:"plan"`
	InitialAmount   int64       `json:"initialAmount"`
	StartDate       *time.Time  `json:"startDate"`
	TrialDays       *int        `json:"trialDays"`
	PaymentMethodID *string     `json:"paymentMethodID"`
}

type planRequest struct {
	Name             string  `json:"name"`
	Vendor           string  `json:"vendor"`
	Description      *string `json:"description"`
	BillingFrequency string  `json:"billingFrequency"`
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
