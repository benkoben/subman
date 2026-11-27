package web

import (
	"net/http"

	"github.com/google/uuid"
)

func (s *server) handleProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.PathValue("userID"))
	if err != nil {
		s.renderError(w, http.StatusBadRequest, "The User ID must be a UUID.")
		return
	}

	profile, err := s.service.Profile(r.Context(), userID)
	if err != nil {
		s.renderServiceError(w, err)
		return
	}
	page, err := newProfilePage(profile, s.clock())
	if err != nil {
		s.renderServiceError(w, err)
		return
	}
	s.render(w, http.StatusOK, page)
}
