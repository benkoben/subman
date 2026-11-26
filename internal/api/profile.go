package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"subman/internal/app"
)

func handleGetProfile(svc *app.Service, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		_ = encode(w, http.StatusOK, profileResponse{
			User:          toUserResponse(p.User),
			Cards:         cards,
			Subscriptions: subs,
		})
	})
}
