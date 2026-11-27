package web

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"

	"subman/internal/app"
	"subman/internal/payment"
)

func (s *server) handleNewCard(w http.ResponseWriter, r *http.Request) {
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
		"cardType":    {string(payment.Debit)},
		"expiryYear":  {strconv.Itoa(now.Year())},
		"expiryMonth": {strconv.Itoa(int(now.Month()))},
	}
	s.render(w, http.StatusOK, cardFormPage{baseData: newBase("Add a Card", "card-form", profile.User), Form: form})
}

func (s *server) handleCreateCard(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.parseUserID(w, r)
	if !ok {
		return
	}
	form, err := parseForm(w, r)
	if err != nil {
		s.renderCardFormError(w, r, userID, form, err)
		return
	}

	name, err := requiredValue(form, "name", "Card name")
	if err != nil {
		s.renderCardFormError(w, r, userID, form, err)
		return
	}
	cardType, err := requiredValue(form, "cardType", "Card type")
	if err != nil {
		s.renderCardFormError(w, r, userID, form, err)
		return
	}
	expiryYear, err := positiveInt(form, "expiryYear", "Expiry year")
	if err != nil {
		s.renderCardFormError(w, r, userID, form, err)
		return
	}
	expiryMonth, err := positiveInt(form, "expiryMonth", "Expiry month")
	if err != nil {
		s.renderCardFormError(w, r, userID, form, err)
		return
	}

	_, err = s.service.CreateCard(r.Context(), userID, name, payment.CardType(cardType), expiryYear, time.Month(expiryMonth))
	if err != nil {
		if errors.Is(err, app.ErrInvalidInput) {
			s.renderCardFormError(w, r, userID, form, err)
		} else {
			s.renderServiceError(w, err)
		}
		return
	}
	http.Redirect(w, r, profileURL(userID.String())+"#wallet", http.StatusSeeOther)
}

func (s *server) renderCardFormError(w http.ResponseWriter, r *http.Request, userID uuid.UUID, form url.Values, formErr error) {
	if form == nil {
		form = make(url.Values)
	}
	profile, err := s.service.Profile(r.Context(), userID)
	if err != nil {
		s.renderServiceError(w, err)
		return
	}
	page := cardFormPage{baseData: newBase("Add a Card", "card-form", profile.User), Form: form}
	page.Error = formErr.Error()
	s.render(w, http.StatusBadRequest, page)
}

func (s *server) handleRemoveCard(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.parseUserID(w, r)
	if !ok {
		return
	}
	cardID, err := uuid.Parse(r.PathValue("cardID"))
	if err != nil {
		s.renderError(w, http.StatusBadRequest, "The Card ID must be a UUID.")
		return
	}

	profile, err := s.service.Profile(r.Context(), userID)
	if err != nil {
		s.renderServiceError(w, err)
		return
	}
	var card *payment.Card
	for _, candidate := range profile.Cards {
		if candidate.ID() == cardID {
			card = candidate
			break
		}
	}
	if card == nil {
		s.renderError(w, http.StatusNotFound, "The requested Card could not be found in this Wallet.")
		return
	}

	now := s.clock()
	form := url.Values{"removedAt": {inputTime(now)}}
	if r.Method == http.MethodPost {
		form, err = parseForm(w, r)
		if err == nil {
			var removedAt time.Time
			removedAt, err = utcDateTime(form, "removedAt", "Removal time")
			if err == nil {
				_, serviceErr := s.service.RemoveCard(r.Context(), userID, cardID, removedAt)
				if serviceErr != nil {
					s.renderServiceError(w, serviceErr)
					return
				}
			}
		}
		if err == nil {
			http.Redirect(w, r, profileURL(userID.String())+"#wallet", http.StatusSeeOther)
			return
		}
	}

	view := newCardView(card, now)
	page := confirmationPage{
		baseData: newBase("Remove Card", "card-removal", profile.User),
		Form:     form,
		Card:     &view,
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

func (s *server) parseUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userID, err := uuid.Parse(r.PathValue("userID"))
	if err != nil {
		s.renderError(w, http.StatusBadRequest, "The User ID must be a UUID.")
		return uuid.Nil, false
	}
	return userID, true
}
