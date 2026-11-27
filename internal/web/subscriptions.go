package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"subman/internal/app"
	"subman/internal/plan"
	"subman/internal/subscription"
)

func (s *server) handleNewSubscription(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.parseUserID(w, r)
	if !ok {
		return
	}
	profile, err := s.service.Profile(r.Context(), userID)
	if err != nil {
		s.renderServiceError(w, err)
		return
	}
	now := s.clock()
	form := url.Values{
		"billingFrequency": {string(plan.Monthly)},
		"startDate":        {inputTime(now)},
	}
	s.render(w, http.StatusOK, newSubscriptionFormPage(profile, form, now))
}

func (s *server) handleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.parseUserID(w, r)
	if !ok {
		return
	}
	form, err := parseForm(w, r)
	if err != nil {
		s.renderSubscriptionFormError(w, r, userID, form, err)
		return
	}

	input, err := subscriptionInput(form)
	if err != nil {
		s.renderSubscriptionFormError(w, r, userID, form, err)
		return
	}
	created, err := s.service.CreateSubscription(r.Context(), userID, input)
	if err != nil {
		if errors.Is(err, app.ErrInvalidInput) {
			s.renderSubscriptionFormError(w, r, userID, form, err)
		} else {
			s.renderServiceError(w, err)
		}
		return
	}
	http.Redirect(w, r, subscriptionURL(userID.String(), created.ID().String()), http.StatusSeeOther)
}

func subscriptionInput(form url.Values) (app.SubscriptionInput, error) {
	planName, err := requiredValue(form, "planName", "Plan name")
	if err != nil {
		return app.SubscriptionInput{}, err
	}
	vendor, err := requiredValue(form, "vendor", "Vendor")
	if err != nil {
		return app.SubscriptionInput{}, err
	}
	frequency, err := requiredValue(form, "billingFrequency", "Billing frequency")
	if err != nil {
		return app.SubscriptionInput{}, err
	}
	amount, err := positiveInt64(form, "initialAmount", "Opening amount")
	if err != nil {
		return app.SubscriptionInput{}, err
	}
	startDate, err := utcDateTime(form, "startDate", "Start date")
	if err != nil {
		return app.SubscriptionInput{}, err
	}

	input := app.SubscriptionInput{
		PlanName:         planName,
		PlanVendor:       vendor,
		BillingFrequency: plan.BillingFrequency(frequency),
		InitialAmount:    amount,
		StartDate:        startDate,
	}
	if description := strings.TrimSpace(form.Get("description")); description != "" {
		input.PlanDescription = &description
	}
	if trial := strings.TrimSpace(form.Get("trialDays")); trial != "" {
		trialDays, err := positiveInt(form, "trialDays", "Trial length")
		if err != nil {
			return app.SubscriptionInput{}, err
		}
		input.TrialDays = &trialDays
	}
	if paymentMethod := strings.TrimSpace(form.Get("paymentMethodID")); paymentMethod != "" {
		paymentMethodID, err := uuid.Parse(paymentMethod)
		if err != nil {
			return app.SubscriptionInput{}, fmt.Errorf("payment method must be a valid Card")
		}
		input.PaymentMethodID = &paymentMethodID
	}
	return input, nil
}

func (s *server) renderSubscriptionFormError(w http.ResponseWriter, r *http.Request, userID uuid.UUID, form url.Values, formErr error) {
	if form == nil {
		form = make(url.Values)
	}
	profile, err := s.service.Profile(r.Context(), userID)
	if err != nil {
		s.renderServiceError(w, err)
		return
	}
	page := newSubscriptionFormPage(profile, form, s.clock())
	page.Error = formErr.Error()
	s.render(w, http.StatusBadRequest, page)
}

func newSubscriptionFormPage(profile app.Profile, form url.Values, now time.Time) subscriptionFormPage {
	cards := make([]cardView, 0, len(profile.Cards))
	for _, card := range profile.Cards {
		cards = append(cards, newCardView(card, now))
	}
	sort.Slice(cards, func(i, j int) bool { return cards[i].Name < cards[j].Name })
	return subscriptionFormPage{
		baseData: newBase("Add a Subscription", "subscription-form", profile.User),
		Form:     form,
		Cards:    cards,
	}
}

func (s *server) handleSubscription(w http.ResponseWriter, r *http.Request) {
	userID, subscriptionID, ok := s.parseSubscriptionIDs(w, r)
	if !ok {
		return
	}
	profile, sub := s.findSubscription(w, r, userID, subscriptionID)
	if sub == nil {
		return
	}

	cardNames := make(map[string]string, len(profile.Cards))
	for _, card := range profile.Cards {
		cardNames[card.ID().String()] = card.Name()
	}
	view, err := newSubscriptionView(sub, s.clock(), cardNames)
	if err != nil {
		s.renderServiceError(w, err)
		return
	}
	s.render(w, http.StatusOK, subscriptionPage{
		baseData:     newBase(sub.Plan().Name, "subscription", profile.User),
		Subscription: view,
	})
}

func (s *server) handlePriceInterval(w http.ResponseWriter, r *http.Request) {
	userID, subscriptionID, ok := s.parseSubscriptionIDs(w, r)
	if !ok {
		return
	}
	profile, sub := s.findSubscription(w, r, userID, subscriptionID)
	if sub == nil {
		return
	}

	now := s.clock()
	view, err := newSubscriptionView(sub, now, nil)
	if err != nil {
		s.renderServiceError(w, err)
		return
	}
	form := url.Values{"from": {inputTime(now)}}
	if r.Method == http.MethodPost {
		form, err = parseForm(w, r)
		if err == nil {
			var from time.Time
			from, err = utcDateTime(form, "from", "Effective from")
			if err == nil {
				var amount int64
				amount, err = positiveInt64(form, "amount", "Amount")
				if err == nil {
					_, serviceErr := s.service.AddPriceInterval(r.Context(), userID, subscriptionID, from, amount)
					if serviceErr != nil && !errors.Is(serviceErr, app.ErrInvalidInput) {
						s.renderServiceError(w, serviceErr)
						return
					}
					err = serviceErr
				}
			}
		}
		if err == nil {
			http.Redirect(w, r, subscriptionURL(userID.String(), subscriptionID.String())+"#pricing", http.StatusSeeOther)
			return
		}
	}

	page := intervalPage{
		baseData:     newBase("Add a Price Interval", "interval-form", profile.User),
		Subscription: view,
		Form:         form,
	}
	if err != nil {
		page.Error = err.Error()
	}
	status := http.StatusOK
	if r.Method == http.MethodPost {
		status = http.StatusBadRequest
	}
	s.render(w, status, page)
}

func (s *server) handleCancellation(w http.ResponseWriter, r *http.Request) {
	userID, subscriptionID, ok := s.parseSubscriptionIDs(w, r)
	if !ok {
		return
	}
	profile, sub := s.findSubscription(w, r, userID, subscriptionID)
	if sub == nil {
		return
	}

	now := s.clock()
	view, err := newSubscriptionView(sub, now, nil)
	if err != nil {
		s.renderServiceError(w, err)
		return
	}
	form := url.Values{"cancelledAt": {inputTime(now)}}
	if r.Method == http.MethodPost {
		form, err = parseForm(w, r)
		if err == nil {
			var cancelledAt time.Time
			cancelledAt, err = utcDateTime(form, "cancelledAt", "Cancellation time")
			if err == nil {
				_, serviceErr := s.service.CancelSubscription(r.Context(), userID, subscriptionID, cancelledAt)
				if serviceErr != nil && !errors.Is(serviceErr, app.ErrInvalidInput) {
					s.renderServiceError(w, serviceErr)
					return
				}
				err = serviceErr
			}
		}
		if err == nil {
			http.Redirect(w, r, subscriptionURL(userID.String(), subscriptionID.String()), http.StatusSeeOther)
			return
		}
	}

	page := confirmationPage{
		baseData:     newBase("Cancel Subscription", "cancellation", profile.User),
		Form:         form,
		Subscription: &view,
	}
	if err != nil {
		page.Error = err.Error()
	}
	status := http.StatusOK
	if r.Method == http.MethodPost {
		status = http.StatusBadRequest
	}
	s.render(w, status, page)
}

func (s *server) parseSubscriptionIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	userID, ok := s.parseUserID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	subscriptionID, err := uuid.Parse(r.PathValue("subscriptionID"))
	if err != nil {
		s.renderError(w, http.StatusBadRequest, "The Subscription ID must be a UUID.")
		return uuid.Nil, uuid.Nil, false
	}
	return userID, subscriptionID, true
}

func (s *server) findSubscription(w http.ResponseWriter, r *http.Request, userID, subscriptionID uuid.UUID) (app.Profile, *subscription.Subscription) {
	profile, err := s.service.Profile(r.Context(), userID)
	if err != nil {
		s.renderServiceError(w, err)
		return app.Profile{}, nil
	}
	for _, sub := range profile.Subscriptions {
		if sub.ID() == subscriptionID {
			return profile, sub
		}
	}
	s.renderError(w, http.StatusNotFound, "The requested Subscription could not be found in this Profile.")
	return app.Profile{}, nil
}
