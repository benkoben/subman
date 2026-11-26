package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"subman/internal/app"
	"subman/internal/payment"
)

func handleCreateCard(svc *app.Service, clock func() time.Time) http.Handler {
	type request struct {
		Name        string `json:"name"`
		CardType    string `json:"cardType"`
		ExpiryYear  int    `json:"expiryYear"`
		ExpiryMonth int    `json:"expiryMonth"`
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

		c, err := svc.CreateCard(r.Context(), userID, req.Name, payment.CardType(req.CardType),
			req.ExpiryYear, time.Month(req.ExpiryMonth))
		if err != nil {
			writeAppError(w, err)
			return
		}

		_ = encode(w, http.StatusCreated, toCardResponse(c, clock()))
	})
}

func handleListCards(svc *app.Service, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		_ = encode(w, http.StatusOK, out)
	})
}

func handleRemoveCard(svc *app.Service, clock func() time.Time) http.Handler {
	type request struct {
		RemovedAt *time.Time `json:"removedAt"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		req, err := decode[request](r)
		if err != nil {
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

		_ = encode(w, http.StatusOK, toCardResponse(c, clock()))
	})
}
