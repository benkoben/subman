package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const missingUserID = "7f3c2a1e-9b4d-4e8f-a6c5-1d2e3f4a5b6c"

func createUserID(t *testing.T, h http.Handler) string {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users",
		strings.NewReader(`{"name":"Erika Lindqvist","email":"erika@example.com"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("create user body is not JSON: %v", err)
	}
	return got.ID
}

func TestPostCardsCreatesCard(t *testing.T) {
	h := newTestHandler(t)
	userID := createUserID(t, h)

	req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/cards", strings.NewReader(
		`{"name":"Everyday debit","cardType":"debit","expiryYear":2099,"expiryMonth":8}`))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var got struct {
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		CardType    string  `json:"cardType"`
		ExpiryYear  int     `json:"expiryYear"`
		ExpiryMonth int     `json:"expiryMonth"`
		RemovedAt   *string `json:"removedAt"`
		Expired     bool    `json:"expired"`
		Active      bool    `json:"active"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v; body = %s", err, rec.Body.String())
	}
	if got.ID == "" || got.Name != "Everyday debit" || got.CardType != "debit" ||
		got.ExpiryYear != 2099 || got.ExpiryMonth != 8 {
		t.Errorf("card = %+v, want Everyday debit/debit/2099-08", got)
	}
	if got.RemovedAt != nil {
		t.Errorf("removedAt = %q, want null", *got.RemovedAt)
	}
	if got.Expired || !got.Active {
		t.Errorf("expired/active = %t/%t, want false/true", got.Expired, got.Active)
	}
}

func TestPostCardsUnknownUserReturnsNotFound(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/users/"+missingUserID+"/cards", strings.NewReader(
		`{"name":"Everyday debit","cardType":"debit","expiryYear":2099,"expiryMonth":8}`))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestPostCardsRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		userID string
		body   string
	}{
		{
			name:   "malformed user id",
			userID: "not-a-uuid",
			body:   `{"name":"Everyday debit","cardType":"debit","expiryYear":2099,"expiryMonth":8}`,
		},
		{
			name: "unknown card type",
			body: `{"name":"Everyday debit","cardType":"prepaid","expiryYear":2099,"expiryMonth":8}`,
		},
		{
			name: "expiry month above twelve",
			body: `{"name":"Everyday debit","cardType":"debit","expiryYear":2099,"expiryMonth":13}`,
		},
		{
			name: "unknown field",
			body: `{"name":"Everyday debit","cardType":"debit","expiryYear":2099,"expiryMonth":8,"status":"active"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(t)
			userID := tt.userID
			if userID == "" {
				userID = createUserID(t, h)
			}

			req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/cards", strings.NewReader(tt.body))
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

func TestPostCardsReportsDerivedExpiry(t *testing.T) {
	h := newTestHandler(t)
	userID := createUserID(t, h)

	req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/cards", strings.NewReader(
		`{"name":"Old credit","cardType":"credit","expiryYear":2020,"expiryMonth":1}`))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got struct {
		Expired bool `json:"expired"`
		Active  bool `json:"active"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v; body = %s", err, rec.Body.String())
	}
	if !got.Expired || got.Active {
		t.Errorf("expired/active = %t/%t, want true/false", got.Expired, got.Active)
	}
}

func TestGetCardsListsOnlyOwnersCards(t *testing.T) {
	h := newTestHandler(t)
	erika := createUserID(t, h)
	grace := createUserID(t, h)
	empty := createUserID(t, h)

	createCard := func(userID, name string) string {
		t.Helper()
		rec := httptest.NewRecorder()
		body := `{"name":"` + name + `","cardType":"debit","expiryYear":2099,"expiryMonth":8}`
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/cards", strings.NewReader(body)))
		if rec.Code != http.StatusCreated {
			t.Fatalf("create card status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body.String())
		}
		var got struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("create card body is not JSON: %v", err)
		}
		return got.ID
	}

	erikaCards := map[string]bool{
		createCard(erika, "Everyday debit"): true,
		createCard(erika, "Travel credit"):  true,
	}
	createCard(grace, "Grace's debit")

	listCards := func(userID string) []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} {
		t.Helper()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/"+userID+"/cards", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		var list []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
			t.Fatalf("GET body is not a JSON array: %v; body = %s", err, rec.Body.String())
		}
		return list
	}

	got := map[string]bool{}
	for _, c := range listCards(erika) {
		got[c.ID] = true
	}
	if len(got) != len(erikaCards) {
		t.Fatalf("listed %d cards, want %d", len(got), len(erikaCards))
	}
	for id := range erikaCards {
		if !got[id] {
			t.Errorf("card %s missing from owner's list", id)
		}
	}

	if list := listCards(empty); list == nil || len(list) != 0 {
		t.Errorf("empty wallet list = %v, want non-nil empty slice", list)
	}
}

func TestGetCardsUnknownUserReturnsNotFound(t *testing.T) {
	h := newTestHandler(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/"+missingUserID+"/cards", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func createCardID(t *testing.T, h http.Handler, userID, name string) string {
	t.Helper()

	rec := httptest.NewRecorder()
	body := `{"name":"` + name + `","cardType":"debit","expiryYear":2099,"expiryMonth":8}`
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/cards", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create card status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("create card body is not JSON: %v", err)
	}
	return got.ID
}

func TestPostCardRemovalRetiresCard(t *testing.T) {
	h := newTestHandler(t)
	userID := createUserID(t, h)
	cardID := createCardID(t, h, userID, "Everyday debit")

	req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/cards/"+cardID+"/removal", strings.NewReader(
		`{"removedAt":"2026-03-10T12:00:00Z"}`))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got struct {
		ID        string  `json:"id"`
		RemovedAt *string `json:"removedAt"`
		Active    bool    `json:"active"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v; body = %s", err, rec.Body.String())
	}
	if got.ID != cardID {
		t.Errorf("id = %q, want %q", got.ID, cardID)
	}
	if got.RemovedAt == nil || *got.RemovedAt != "2026-03-10T12:00:00Z" {
		t.Errorf("removedAt = %v, want 2026-03-10T12:00:00Z", got.RemovedAt)
	}
	if got.Active {
		t.Errorf("active = true, want false after removal")
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/"+userID+"/cards", nil))
	var list []struct {
		ID        string  `json:"id"`
		RemovedAt *string `json:"removedAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("GET body is not a JSON array: %v; body = %s", err, rec.Body.String())
	}
	if len(list) != 1 || list[0].ID != cardID || list[0].RemovedAt == nil {
		t.Errorf("GET after removal = %s, want the removed card still listed", rec.Body.String())
	}
}

func TestPostCardRemovalKeepsEarliestTimestamp(t *testing.T) {
	h := newTestHandler(t)
	userID := createUserID(t, h)
	cardID := createCardID(t, h, userID, "Everyday debit")
	path := "/users/" + userID + "/cards/" + cardID + "/removal"

	for _, body := range []string{
		`{"removedAt":"2026-03-10T12:00:00Z"}`,
		`{"removedAt":"2026-06-01T08:00:00Z"}`,
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/"+userID+"/cards", nil))
	var list []struct {
		RemovedAt *string `json:"removedAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("GET body is not a JSON array: %v; body = %s", err, rec.Body.String())
	}
	if len(list) != 1 || list[0].RemovedAt == nil || *list[0].RemovedAt != "2026-03-10T12:00:00Z" {
		t.Errorf("GET after repeated removal = %s, want removedAt 2026-03-10T12:00:00Z", rec.Body.String())
	}
}

func TestPostCardRemovalRejectsInvalidRequests(t *testing.T) {
	const validBody = `{"removedAt":"2026-03-10T12:00:00Z"}`

	tests := []struct {
		name     string
		userID   string
		cardID   string
		body     string
		wantCode int
	}{
		{name: "malformed user id", userID: "not-a-uuid", cardID: "CARD", body: validBody, wantCode: http.StatusBadRequest},
		{name: "malformed card id", cardID: "not-a-uuid", body: validBody, wantCode: http.StatusBadRequest},
		{name: "missing removedAt", body: `{}`, wantCode: http.StatusBadRequest},
		{name: "invalid removedAt", body: `{"removedAt":"next Tuesday"}`, wantCode: http.StatusBadRequest},
		{name: "unknown user", userID: missingUserID, cardID: "CARD", body: validBody, wantCode: http.StatusNotFound},
		{name: "unknown card", cardID: missingUserID, body: validBody, wantCode: http.StatusNotFound},
		{name: "card under another user", userID: "OTHER", cardID: "CARD", body: validBody, wantCode: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(t)
			userID := createUserID(t, h)
			cardID := createCardID(t, h, userID, "Everyday debit")
			if tt.userID == "" {
				tt.userID = userID
			}
			if tt.userID == "OTHER" {
				tt.userID = createUserID(t, h)
			}
			if tt.cardID == "" || tt.cardID == "CARD" {
				tt.cardID = cardID
			}

			path := "/users/" + tt.userID + "/cards/" + tt.cardID + "/removal"
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(tt.body)))

			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.wantCode, rec.Body.String())
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
