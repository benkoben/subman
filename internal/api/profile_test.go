package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"subman/internal/api"
)

type profileBody struct {
	User struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"user"`
	Cards         json.RawMessage `json:"cards"`
	Subscriptions json.RawMessage `json:"subscriptions"`
}

func getProfile(t *testing.T, h http.Handler, userID string) (*httptest.ResponseRecorder, profileBody) {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/"+userID+"/profile", nil))

	var got profileBody
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("profile body is not JSON: %v; body = %s", err, rec.Body.String())
		}
	}
	return rec, got
}

func TestGetProfileReadsClockOnceForEveryDerivedField(t *testing.T) {
	feb := time.Date(2026, time.February, 20, 0, 0, 0, 0, time.UTC)
	june := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)

	var armed bool
	var reads int
	h := newTestHandler(t, api.WithClock(func() time.Time {
		if !armed {
			return june
		}
		reads++
		if reads == 1 {
			return feb
		}
		return june
	}))
	userID := createUserID(t, h)
	createSubscriptionID(t, h, userID, "Streaming Premium")
	createSubscriptionID(t, h, userID, "Cloud Storage")
	createCardID(t, h, userID, "Everyday debit")

	armed = true
	rec, got := getProfile(t, h, userID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if reads != 1 {
		t.Errorf("clock read %d times, want exactly once per profile", reads)
	}
	var subs []subscriptionBody
	if err := json.Unmarshal(got.Subscriptions, &subs); err != nil || len(subs) != 2 {
		t.Fatalf("subscriptions = %s, want two", got.Subscriptions)
	}
	for _, s := range subs {
		if s.TotalPaid.Amount != 60 {
			t.Errorf("subscription %s totalPaid = %d, want 60 at the single evaluation time", s.ID, s.TotalPaid.Amount)
		}
	}
}

func TestGetProfileShowsRemovedCardAndCancelledSubscription(t *testing.T) {
	evaluatedAt := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	h := newTestHandler(t, api.WithClock(func() time.Time { return evaluatedAt }))
	userID := createUserID(t, h)
	cardID := createCardID(t, h, userID, "Everyday debit")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions", strings.NewReader(
		`{`+streamingPlanBody+`,"initialAmount":30,"startDate":"2026-01-10T12:00:00Z","paymentMethodID":"`+cardID+`"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create subscription status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var created subscriptionBody
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("create subscription body is not JSON: %v", err)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/cards/"+cardID+"/removal",
		strings.NewReader(`{"removedAt":"2026-03-01T12:00:00Z"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("card removal status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions/"+created.ID+"/cancellation",
		strings.NewReader(`{"cancelledAt":"2026-02-20T12:00:00Z"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("cancellation status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	rec, got := getProfile(t, h, userID)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var cards []struct {
		ID        string  `json:"id"`
		RemovedAt *string `json:"removedAt"`
		Expired   bool    `json:"expired"`
		Active    bool    `json:"active"`
	}
	if err := json.Unmarshal(got.Cards, &cards); err != nil || len(cards) != 1 {
		t.Fatalf("cards = %s, want one card", got.Cards)
	}
	if cards[0].ID != cardID || cards[0].RemovedAt == nil || *cards[0].RemovedAt != "2026-03-01T12:00:00Z" {
		t.Errorf("card = %+v, want %s removed at 2026-03-01T12:00:00Z", cards[0], cardID)
	}
	if cards[0].Active || cards[0].Expired {
		t.Errorf("card active/expired = %t/%t, want false/false: removed but not expired", cards[0].Active, cards[0].Expired)
	}

	var subs []subscriptionBody
	if err := json.Unmarshal(got.Subscriptions, &subs); err != nil || len(subs) != 1 {
		t.Fatalf("subscriptions = %s, want one subscription", got.Subscriptions)
	}
	s := subs[0]
	if s.ID != created.ID {
		t.Errorf("subscription id = %q, want %s", s.ID, created.ID)
	}
	if s.PaymentMethodID == nil || *s.PaymentMethodID != cardID {
		t.Errorf("paymentMethodID = %v, want preserved reference to %s", s.PaymentMethodID, cardID)
	}
	if s.CancelledAt == nil || *s.CancelledAt != "2026-02-20T12:00:00Z" || s.Phase != "cancelled" {
		t.Errorf("cancelledAt/phase = %v/%q, want 2026-02-20T12:00:00Z/cancelled", s.CancelledAt, s.Phase)
	}
	// Charges on 2026-01-10 and 2026-02-10 only: cancellation stops totalPaid.
	if s.TotalPaid.Amount != 60 || s.TotalPaid.Currency != "SEK" {
		t.Errorf("totalPaid = %+v, want 60 SEK", s.TotalPaid)
	}
}

func TestGetProfileRejectsUnknownAndMalformedUser(t *testing.T) {
	h := newTestHandler(t)

	tests := []struct {
		name   string
		userID string
		status int
	}{
		{name: "malformed user id", userID: "not-a-uuid", status: http.StatusBadRequest},
		{name: "unknown user", userID: missingUserID, status: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, _ := getProfile(t, h, tt.userID)

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.status, rec.Body.String())
			}
		})
	}
}

func TestGetProfileOfEmptyUserReturnsUserAndEmptyCollections(t *testing.T) {
	h := newTestHandler(t)
	userID := createUserID(t, h)

	rec, got := getProfile(t, h, userID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got.User.ID != userID || got.User.Name != "Erika Lindqvist" || got.User.Email != "erika@example.com" {
		t.Errorf("user = %+v, want %s Erika Lindqvist/erika@example.com", got.User, userID)
	}
	if string(got.Cards) != "[]" {
		t.Errorf("cards = %s, want []", got.Cards)
	}
	if string(got.Subscriptions) != "[]" {
		t.Errorf("subscriptions = %s, want []", got.Subscriptions)
	}
}
