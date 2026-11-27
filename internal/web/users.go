package web

import (
	"errors"
	"net/http"
	"net/url"

	"subman/internal/app"
)

func (s *server) handleUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.service.ListUsers(r.Context())
	if err != nil {
		s.renderServiceError(w, err)
		return
	}
	s.render(w, http.StatusOK, newUsersPage(users, make(url.Values)))
}

func (s *server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	form, err := parseForm(w, r)
	if err != nil {
		s.renderUsersFormError(w, r, form, err)
		return
	}

	name, err := requiredValue(form, "name", "Name")
	if err != nil {
		s.renderUsersFormError(w, r, form, err)
		return
	}
	email, err := requiredValue(form, "email", "Email")
	if err != nil {
		s.renderUsersFormError(w, r, form, err)
		return
	}

	created, err := s.service.CreateUser(r.Context(), name, email)
	if err != nil {
		if errors.Is(err, app.ErrInvalidInput) {
			s.renderUsersFormError(w, r, form, err)
		} else {
			s.renderServiceError(w, err)
		}
		return
	}
	http.Redirect(w, r, profileURL(created.ID().String()), http.StatusSeeOther)
}

func (s *server) renderUsersFormError(w http.ResponseWriter, r *http.Request, form url.Values, formErr error) {
	if form == nil {
		form = make(url.Values)
	}
	users, err := s.service.ListUsers(r.Context())
	if err != nil {
		s.renderServiceError(w, err)
		return
	}
	page := newUsersPage(users, form)
	page.Error = formErr.Error()
	s.render(w, http.StatusBadRequest, page)
}
