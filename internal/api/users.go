package api

import (
	"net/http"

	"subman/internal/app"
)

func handleCreateUser(svc *app.Service) http.Handler {
	type request struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := decode[request](r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		u, err := svc.CreateUser(r.Context(), req.Name, req.Email)
		if err != nil {
			writeAppError(w, err)
			return
		}

		_ = encode(w, http.StatusCreated, toUserResponse(u))
	})
}

func handleListUsers(svc *app.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		users, err := svc.ListUsers(r.Context())
		if err != nil {
			writeAppError(w, err)
			return
		}

		out := make([]userResponse, 0, len(users))
		for _, u := range users {
			out = append(out, toUserResponse(u))
		}

		_ = encode(w, http.StatusOK, out)
	})
}
