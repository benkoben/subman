package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type errorResponse struct {
	Error string `json:"error"`
}

// decode decodes one request value strictly: unknown fields and trailing JSON
// values are rejected.
func decode[T any](r *http.Request) (T, error) {
	var v T
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&v); err != nil {
		return v, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return v, errors.New("request body must contain a single JSON value")
	}

	return v, nil
}

func encode[T any](w http.ResponseWriter, status int, v T) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	_ = encode(w, status, errorResponse{Error: msg})
}
