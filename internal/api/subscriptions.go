package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"subman/internal/app"
	"subman/internal/plan"
)

func handleCreateSubscription(svc *app.Service, clock func() time.Time) http.Handler {
	type planRequest struct {
		Name             string  `json:"name"`
		Vendor           string  `json:"vendor"`
		Description      *string `json:"description"`
		BillingFrequency string  `json:"billingFrequency"`
	}
	type request struct {
		Plan            planRequest `json:"plan"`
		InitialAmount   int64       `json:"initialAmount"`
		StartDate       *time.Time  `json:"startDate"`
		TrialDays       *int        `json:"trialDays"`
		PaymentMethodID *string     `json:"paymentMethodID"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := uuid.Parse(r.PathValue("userID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "userID must be a UUID")
			return
		}

		req, err := decode[request](r)
		if err != nil {
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

		_ = encode(w, http.StatusCreated, out)
	})
}

func handleListSubscriptions(svc *app.Service, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		_ = encode(w, http.StatusOK, out)
	})
}

func handleCancelSubscription(svc *app.Service, clock func() time.Time) http.Handler {
	type request struct {
		CancelledAt *time.Time `json:"cancelledAt"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		req, err := decode[request](r)
		if err != nil {
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

		_ = encode(w, http.StatusOK, out)
	})
}

func handleAddPriceInterval(svc *app.Service, clock func() time.Time) http.Handler {
	type request struct {
		From   *time.Time `json:"from"`
		Amount int64      `json:"amount"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		req, err := decode[request](r)
		if err != nil {
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

		_ = encode(w, http.StatusOK, out)
	})
}
