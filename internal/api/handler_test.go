package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"

	"subman/internal/api"
	"subman/internal/app"
	"subman/internal/payment"
	"subman/internal/repository/memory"
	"subman/internal/subscription"
	"subman/internal/user"
)

func newTestHandler(t *testing.T, opts ...api.Option) http.Handler {
	t.Helper()

	users := memory.NewRepository(func(u user.User) string { return u.ID().String() })
	cards := memory.NewRepository(func(c *payment.Card) string { return c.ID().String() })
	subs := memory.NewRepository(func(s *subscription.Subscription) string { return s.ID().String() })

	return api.NewHandler(app.NewService(users, cards, subs), opts...)
}

func TestGetUsersEmptyReturnsEmptyArray(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Errorf("body = %s, want []", body)
	}
}

func TestGetUsersListsCreatedUsersByID(t *testing.T) {
	h := newTestHandler(t)

	created := map[string]string{}
	for _, body := range []string{
		`{"name":"Erika Lindqvist","email":"erika@example.com"}`,
		`{"name":"Grace Hopper","email":"grace@example.com"}`,
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body)))
		if rec.Code != http.StatusCreated {
			t.Fatalf("POST status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body.String())
		}
		var got struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("POST body is not JSON: %v", err)
		}
		created[got.ID] = got.Email
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var list []struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("GET body is not a JSON array: %v; body = %s", err, rec.Body.String())
	}

	got := map[string]string{}
	for _, u := range list {
		got[u.ID] = u.Email
	}
	if diff := cmp.Diff(created, got); diff != "" {
		t.Errorf("GET /users by ID mismatch (-created +listed):\n%s", diff)
	}
}

func TestPostUsersRejectsUnknownField(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(
		`{"name":"Erika Lindqvist","email":"erika@example.com","role":"admin"}`))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	var got struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.Error == "" {
		t.Errorf("body = %s, want {\"error\":...}", rec.Body.String())
	}
}

func TestPostUsersRejectsTrailingJSONValue(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(
		`{"name":"Erika Lindqvist","email":"erika@example.com"} {}`))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestPostUsersRejectsInvalidDomainInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty name", body: `{"name":"","email":"erika@example.com"}`},
		{name: "email without at sign", body: `{"name":"Erika Lindqvist","email":"erika.example.com"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(t)

			req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			var got struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.Error == "" {
				t.Errorf("body = %s, want {\"error\":...}", rec.Body.String())
			}
		})
	}
}

func TestPostUsersCreatesUser(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(
		`{"name":"Erika Lindqvist","email":"erika@example.com"}`))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var got struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v; body = %s", err, rec.Body.String())
	}
	if _, err := uuid.Parse(got.ID); err != nil || got.ID != strings.ToLower(got.ID) {
		t.Errorf("id = %q, want canonical UUID string", got.ID)
	}
	if got.Name != "Erika Lindqvist" || got.Email != "erika@example.com" {
		t.Errorf("name/email = %q/%q, want Erika Lindqvist/erika@example.com", got.Name, got.Email)
	}
}
