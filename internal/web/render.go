package web

import (
	"bytes"
	"errors"
	"net/http"

	"subman/internal/app"
	"subman/internal/repository"
)

type page interface {
	pageBase() baseData
}

func (s *server) render(w http.ResponseWriter, status int, data page) {
	var body bytes.Buffer
	tmpl := s.templates[data.pageBase().Content]
	if tmpl == nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(&body, "base", data); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = body.WriteTo(w)
}

func (s *server) renderServiceError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "Something went wrong. Please try again."
	switch {
	case errors.Is(err, repository.ErrNotFound):
		status = http.StatusNotFound
		message = "The requested record could not be found."
	case errors.Is(err, app.ErrInvalidInput):
		status = http.StatusBadRequest
		message = err.Error()
	}
	s.renderError(w, status, message)
}

func (s *server) renderError(w http.ResponseWriter, status int, message string) {
	base := baseData{Title: http.StatusText(status), Content: "error", Error: message}
	s.render(w, status, struct {
		baseData
		Status int
	}{baseData: base, Status: status})
}

func (s *server) handleNotFound(w http.ResponseWriter, _ *http.Request) {
	s.renderError(w, http.StatusNotFound, "The requested page could not be found.")
}
