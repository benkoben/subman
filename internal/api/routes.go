package api

import (
	"net/http"
	"time"

	"subman/internal/app"
)

func addRoutes(mux *http.ServeMux, svc *app.Service, clock func() time.Time) {
	mux.Handle("GET /healthz", handleHealthz())

	mux.Handle("POST /users", handleCreateUser(svc))
	mux.Handle("GET /users", handleListUsers(svc))

	mux.Handle("POST /users/{userID}/cards", handleCreateCard(svc, clock))
	mux.Handle("GET /users/{userID}/cards", handleListCards(svc, clock))
	mux.Handle("POST /users/{userID}/cards/{cardID}/removal", handleRemoveCard(svc, clock))

	mux.Handle("POST /users/{userID}/subscriptions", handleCreateSubscription(svc, clock))
	mux.Handle("GET /users/{userID}/subscriptions", handleListSubscriptions(svc, clock))
	mux.Handle("POST /users/{userID}/subscriptions/{subscriptionID}/cancellation", handleCancelSubscription(svc, clock))
	mux.Handle("POST /users/{userID}/subscriptions/{subscriptionID}/price-intervals", handleAddPriceInterval(svc, clock))

	mux.Handle("GET /users/{userID}/profile", handleGetProfile(svc, clock))
}

func handleHealthz() http.Handler {
	type response struct {
		Status string `json:"status"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = encode(w, http.StatusOK, response{Status: "ok"})
	})
}
