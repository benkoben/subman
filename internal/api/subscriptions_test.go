package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"

	"subman/internal/api"
)

const streamingPlanBody = `"plan":{"name":"Streaming Premium","vendor":"StreamCo","description":"Two screens","billingFrequency":"monthly"}`

type subscriptionBody struct {
	ID              string  `json:"id"`
	PaymentMethodID *string `json:"paymentMethodID"`
	StartDate       string  `json:"startDate"`
	TrialEnd        *string `json:"trialEnd"`
	CancelledAt     *string `json:"cancelledAt"`
	Plan            struct {
		Name             string  `json:"name"`
		Vendor           string  `json:"vendor"`
		Description      *string `json:"description"`
		BillingFrequency string  `json:"billingFrequency"`
	} `json:"plan"`
	Intervals []struct {
		From   string `json:"from"`
		Amount int64  `json:"amount"`
	} `json:"intervals"`
	Phase     string `json:"phase"`
	TotalPaid struct {
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
	} `json:"totalPaid"`
}

func TestPostSubscriptionsReportsPhaseAndTotalPaidAtClock(t *testing.T) {
	evaluatedAt := time.Date(2026, time.February, 20, 0, 0, 0, 0, time.UTC)
	h := newTestHandler(t, api.WithClock(func() time.Time { return evaluatedAt }))
	userID := createUserID(t, h)

	req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions", strings.NewReader(
		`{`+streamingPlanBody+`,"initialAmount":30,"startDate":"2026-01-10T12:00:00Z"}`))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got subscriptionBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v; body = %s", err, rec.Body.String())
	}
	if got.Phase != "active" {
		t.Errorf("phase = %q, want active", got.Phase)
	}
	if got.TotalPaid.Amount != 60 || got.TotalPaid.Currency != "SEK" {
		t.Errorf("totalPaid = %+v, want 60 SEK: charges on 2026-01-10 and 2026-02-10 at 30 each", got.TotalPaid)
	}
}

func TestPostSubscriptionsAnchorsChargesAtTrialEnd(t *testing.T) {
	evaluatedAt := time.Date(2026, time.February, 20, 0, 0, 0, 0, time.UTC)
	h := newTestHandler(t, api.WithClock(func() time.Time { return evaluatedAt }))
	userID := createUserID(t, h)

	req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions", strings.NewReader(
		`{`+streamingPlanBody+`,"initialAmount":30,"startDate":"2026-01-10T12:00:00Z","trialDays":7}`))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got subscriptionBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v; body = %s", err, rec.Body.String())
	}
	if got.TrialEnd == nil || *got.TrialEnd != "2026-01-17T12:00:00Z" {
		t.Errorf("trialEnd = %v, want 2026-01-17T12:00:00Z", got.TrialEnd)
	}
	if len(got.Intervals) != 1 || got.Intervals[0].From != "2026-01-17T12:00:00Z" || got.Intervals[0].Amount != 30 {
		t.Errorf("intervals = %+v, want the Intro Offer anchored at trial end 2026-01-17T12:00:00Z", got.Intervals)
	}
	if got.Phase != "active" {
		t.Errorf("phase = %q, want active after the trial ended", got.Phase)
	}
	if got.TotalPaid.Amount != 60 || got.TotalPaid.Currency != "SEK" {
		t.Errorf("totalPaid = %+v, want 60 SEK: charges on 2026-01-17 and 2026-02-17 at 30 each", got.TotalPaid)
	}
}

func createSubscriptionID(t *testing.T, h http.Handler, userID, planName string) string {
	t.Helper()

	rec := httptest.NewRecorder()
	body := `{"plan":{"name":"` + planName + `","vendor":"StreamCo","billingFrequency":"monthly"},"initialAmount":30,"startDate":"2026-01-10T12:00:00Z"}`
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create subscription status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got subscriptionBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("create subscription body is not JSON: %v", err)
	}
	return got.ID
}

func TestGetSubscriptionsListsOnlyOwnersSubscriptions(t *testing.T) {
	h := newTestHandler(t)
	erika := createUserID(t, h)
	grace := createUserID(t, h)
	empty := createUserID(t, h)

	erikaIDs := map[string]bool{
		createSubscriptionID(t, h, erika, "Streaming Premium"): true,
		createSubscriptionID(t, h, erika, "Cloud Storage"):     true,
	}
	createSubscriptionID(t, h, grace, "Grace's Music")

	listSubscriptions := func(userID string) []subscriptionBody {
		t.Helper()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/"+userID+"/subscriptions", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		var list []subscriptionBody
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
			t.Fatalf("GET body is not a JSON array: %v; body = %s", err, rec.Body.String())
		}
		return list
	}

	got := map[string]bool{}
	for _, s := range listSubscriptions(erika) {
		got[s.ID] = true
	}
	if diff := cmp.Diff(erikaIDs, got); diff != "" {
		t.Errorf("erika's subscriptions by ID mismatch (-want +got):\n%s", diff)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/"+empty+"/subscriptions", nil))
	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Errorf("empty subscription list body = %s, want []", body)
	}
}

func TestPostSubscriptionsRejectsInvalidInput(t *testing.T) {
	const validBody = `{` + streamingPlanBody + `,"initialAmount":30,"startDate":"2026-01-10T12:00:00Z"}`

	tests := []struct {
		name   string
		userID string
		body   string
	}{
		{name: "malformed user id", userID: "not-a-uuid", body: validBody},
		{name: "missing startDate", body: `{` + streamingPlanBody + `,"initialAmount":30}`},
		{name: "invalid startDate", body: `{` + streamingPlanBody + `,"initialAmount":30,"startDate":"next Tuesday"}`},
		{name: "non-positive amount", body: `{` + streamingPlanBody + `,"initialAmount":0,"startDate":"2026-01-10T12:00:00Z"}`},
		{name: "non-positive trial", body: `{` + streamingPlanBody + `,"initialAmount":30,"startDate":"2026-01-10T12:00:00Z","trialDays":0}`},
		{name: "unknown billing frequency", body: `{"plan":{"name":"Streaming Premium","vendor":"StreamCo","billingFrequency":"weekly"},"initialAmount":30,"startDate":"2026-01-10T12:00:00Z"}`},
		{name: "empty plan name", body: `{"plan":{"name":"","vendor":"StreamCo","billingFrequency":"monthly"},"initialAmount":30,"startDate":"2026-01-10T12:00:00Z"}`},
		{name: "malformed payment method id", body: `{` + streamingPlanBody + `,"initialAmount":30,"startDate":"2026-01-10T12:00:00Z","paymentMethodID":"not-a-uuid"}`},
		{name: "unknown field", body: `{` + streamingPlanBody + `,"initialAmount":30,"startDate":"2026-01-10T12:00:00Z","status":"active"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(t)
			userID := tt.userID
			if userID == "" {
				userID = createUserID(t, h)
			}

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions", strings.NewReader(tt.body)))

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

func TestSubscriptionsUnknownUserReturnsNotFound(t *testing.T) {
	h := newTestHandler(t)
	body := `{` + streamingPlanBody + `,"initialAmount":30,"startDate":"2026-01-10T12:00:00Z"}`

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+missingUserID+"/subscriptions", strings.NewReader(body)))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST status = %d, want %d; body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/"+missingUserID+"/subscriptions", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET status = %d, want %d; body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestPostSubscriptionsAttachesOwnedPaymentMethod(t *testing.T) {
	h := newTestHandler(t)
	userID := createUserID(t, h)
	cardID := createCardID(t, h, userID, "Everyday debit")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions", strings.NewReader(
		`{`+streamingPlanBody+`,"initialAmount":30,"startDate":"2026-01-10T12:00:00Z","paymentMethodID":"`+cardID+`"}`)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got subscriptionBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v; body = %s", err, rec.Body.String())
	}
	if got.PaymentMethodID == nil || *got.PaymentMethodID != cardID {
		t.Errorf("paymentMethodID = %v, want %s", got.PaymentMethodID, cardID)
	}
}

func TestPostSubscriptionsRejectsPaymentMethodNotOwnedByUser(t *testing.T) {
	tests := []struct {
		name          string
		paymentMethod func(t *testing.T, h http.Handler) string
	}{
		{
			name: "card under another user",
			paymentMethod: func(t *testing.T, h http.Handler) string {
				other := createUserID(t, h)
				return createCardID(t, h, other, "Grace's debit")
			},
		},
		{
			name: "unknown card",
			paymentMethod: func(t *testing.T, h http.Handler) string {
				return missingUserID
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(t)
			userID := createUserID(t, h)
			cardID := tt.paymentMethod(t, h)

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions", strings.NewReader(
				`{`+streamingPlanBody+`,"initialAmount":30,"startDate":"2026-01-10T12:00:00Z","paymentMethodID":"`+cardID+`"}`)))

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
			}
		})
	}
}

func TestPostSubscriptionsRejectsZeroPaymentMethodID(t *testing.T) {
	h := newTestHandler(t)
	userID := createUserID(t, h)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions", strings.NewReader(
		`{`+streamingPlanBody+`,"initialAmount":30,"startDate":"2026-01-10T12:00:00Z","paymentMethodID":"00000000-0000-0000-0000-000000000000"}`)))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestPostSubscriptionCancellationCancelsSubscription(t *testing.T) {
	evaluatedAt := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	h := newTestHandler(t, api.WithClock(func() time.Time { return evaluatedAt }))
	userID := createUserID(t, h)
	subID := createSubscriptionID(t, h, userID, "Streaming Premium")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions/"+subID+"/cancellation",
		strings.NewReader(`{"cancelledAt":"2026-02-20T12:00:00Z"}`)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got subscriptionBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v; body = %s", err, rec.Body.String())
	}
	if got.ID != subID {
		t.Errorf("id = %q, want %s", got.ID, subID)
	}
	if got.CancelledAt == nil || *got.CancelledAt != "2026-02-20T12:00:00Z" {
		t.Errorf("cancelledAt = %v, want 2026-02-20T12:00:00Z", got.CancelledAt)
	}
	if got.Phase != "cancelled" {
		t.Errorf("phase = %q, want cancelled", got.Phase)
	}
	// Worked example: monthly 30 from 2026-01-10, cancelled 2026-02-20. Only
	// the 2026-01-10 and 2026-02-10 charges were incurred; the clock at June
	// would otherwise show 150.
	if got.TotalPaid.Amount != 60 || got.TotalPaid.Currency != "SEK" {
		t.Errorf("totalPaid = %+v, want 60 SEK", got.TotalPaid)
	}
}

func TestPostSubscriptionCancellationRejectsInvalidInput(t *testing.T) {
	const validBody = `{"cancelledAt":"2026-02-20T12:00:00Z"}`

	tests := []struct {
		name      string
		badUserID bool
		badSubID  bool
		body      string
	}{
		{name: "malformed user id", badUserID: true, body: validBody},
		{name: "malformed subscription id", badSubID: true, body: validBody},
		{name: "missing cancelledAt", body: `{}`},
		{name: "invalid cancelledAt", body: `{"cancelledAt":"next Tuesday"}`},
		{name: "unknown field", body: `{"cancelledAt":"2026-02-20T12:00:00Z","reason":"too expensive"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(t)
			userID := createUserID(t, h)
			subID := createSubscriptionID(t, h, userID, "Streaming Premium")
			if tt.badUserID {
				userID = "not-a-uuid"
			}
			if tt.badSubID {
				subID = "not-a-uuid"
			}

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions/"+subID+"/cancellation", strings.NewReader(tt.body)))

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

func TestPostSubscriptionPriceIntervalAddsIntervalAndReportsTotalPaid(t *testing.T) {
	evaluatedAt := time.Date(2026, time.February, 20, 0, 0, 0, 0, time.UTC)
	h := newTestHandler(t, api.WithClock(func() time.Time { return evaluatedAt }))
	userID := createUserID(t, h)
	subID := createSubscriptionID(t, h, userID, "Streaming Premium")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions/"+subID+"/price-intervals",
		strings.NewReader(`{"from":"2026-02-10T12:00:00Z","amount":50}`)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got subscriptionBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v; body = %s", err, rec.Body.String())
	}
	if got.ID != subID {
		t.Errorf("id = %q, want %s", got.ID, subID)
	}
	wantIntervals := []struct {
		From   string `json:"from"`
		Amount int64  `json:"amount"`
	}{
		{From: "2026-01-10T12:00:00Z", Amount: 30},
		{From: "2026-02-10T12:00:00Z", Amount: 50},
	}
	if diff := cmp.Diff(wantIntervals, got.Intervals); diff != "" {
		t.Errorf("intervals mismatch (-want +got):\n%s", diff)
	}
	// Worked example: monthly 30 from 2026-01-10, then 50 from 2026-02-10,
	// evaluated 2026-02-20: charges of 30 on Jan 10 and 50 on Feb 10.
	if got.TotalPaid.Amount != 80 || got.TotalPaid.Currency != "SEK" {
		t.Errorf("totalPaid = %+v, want 80 SEK", got.TotalPaid)
	}
	if got.Phase != "active" {
		t.Errorf("phase = %q, want active", got.Phase)
	}
}

func TestPostSubscriptionPriceIntervalRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name      string
		badUserID bool
		badSubID  bool
		body      string
	}{
		{name: "malformed user id", badUserID: true, body: `{"from":"2026-02-10T12:00:00Z","amount":50}`},
		{name: "malformed subscription id", badSubID: true, body: `{"from":"2026-02-10T12:00:00Z","amount":50}`},
		{name: "missing from", body: `{"amount":50}`},
		{name: "invalid from", body: `{"from":"next Tuesday","amount":50}`},
		{name: "non-positive amount", body: `{"from":"2026-02-10T12:00:00Z","amount":0}`},
		{name: "from before first charge", body: `{"from":"2026-01-09T12:00:00Z","amount":50}`},
		{name: "duplicate from", body: `{"from":"2026-01-10T12:00:00Z","amount":50}`},
		{name: "unknown field", body: `{"from":"2026-02-10T12:00:00Z","amount":50,"currency":"EUR"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(t)
			ownerID := createUserID(t, h)
			subID := createSubscriptionID(t, h, ownerID, "Streaming Premium")
			userID := ownerID
			if tt.badUserID {
				userID = "not-a-uuid"
			}
			if tt.badSubID {
				subID = "not-a-uuid"
			}

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions/"+subID+"/price-intervals", strings.NewReader(tt.body)))

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			var got struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.Error == "" {
				t.Errorf("body = %s, want {\"error\":...}", rec.Body.String())
			}

			if n := len(listSubscriptionsFor(t, h, ownerID)[0].Intervals); n != 1 {
				t.Errorf("intervals count = %d after rejected request, want unchanged single interval", n)
			}
		})
	}
}

func TestPostSubscriptionPriceIntervalReturnsNotFoundForUnownedSubscription(t *testing.T) {
	const body = `{"from":"2026-02-10T12:00:00Z","amount":50}`

	h := newTestHandler(t)
	owner := createUserID(t, h)
	subID := createSubscriptionID(t, h, owner, "Streaming Premium")
	stranger := createUserID(t, h)

	tests := []struct {
		name, userID, subID string
	}{
		{name: "unknown user", userID: missingUserID, subID: subID},
		{name: "unknown subscription", userID: owner, subID: missingUserID},
		{name: "subscription owned by another user", userID: stranger, subID: subID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+tt.userID+"/subscriptions/"+tt.subID+"/price-intervals", strings.NewReader(body)))

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
			}
		})
	}

	s := listSubscriptionsFor(t, h, owner)[0]
	if len(s.Intervals) != 1 {
		t.Errorf("owner's intervals = %+v after rejected requests, want unchanged single interval", s.Intervals)
	}
}

func TestPostSubscriptionPriceIntervalKeepsHistorySorted(t *testing.T) {
	evaluatedAt := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	h := newTestHandler(t, api.WithClock(func() time.Time { return evaluatedAt }))
	userID := createUserID(t, h)
	subID := createSubscriptionID(t, h, userID, "Streaming Premium")

	var got subscriptionBody
	for _, body := range []string{
		`{"from":"2026-03-10T12:00:00Z","amount":40}`,
		`{"from":"2026-02-10T12:00:00Z","amount":50}`,
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions/"+subID+"/price-intervals",
			strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("body is not JSON: %v; body = %s", err, rec.Body.String())
		}
	}

	wantFroms := []string{"2026-01-10T12:00:00Z", "2026-02-10T12:00:00Z", "2026-03-10T12:00:00Z"}
	gotFroms := make([]string, 0, len(got.Intervals))
	for _, iv := range got.Intervals {
		gotFroms = append(gotFroms, iv.From)
	}
	if diff := cmp.Diff(wantFroms, gotFroms); diff != "" {
		t.Errorf("interval froms mismatch (-want +got):\n%s", diff)
	}
	// Charges at 30 (Jan 10), 50 (Feb 10) and 40 (Mar 10) at evaluation time 2026-03-20.
	if got.TotalPaid.Amount != 120 {
		t.Errorf("totalPaid = %d, want 120", got.TotalPaid.Amount)
	}
}

func TestPostSubscriptionCancellationKeepsEarliestTimestamp(t *testing.T) {
	h := newTestHandler(t)
	userID := createUserID(t, h)
	subID := createSubscriptionID(t, h, userID, "Streaming Premium")

	for _, cancelledAt := range []string{"2026-02-20T12:00:00Z", "2026-03-15T12:00:00Z"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions/"+subID+"/cancellation",
			strings.NewReader(`{"cancelledAt":"`+cancelledAt+`"}`)))

		if rec.Code != http.StatusOK {
			t.Fatalf("cancel at %s: status = %d, want %d; body = %s", cancelledAt, rec.Code, http.StatusOK, rec.Body.String())
		}
	}

	s := listSubscriptionsFor(t, h, userID)[0]
	if s.CancelledAt == nil || *s.CancelledAt != "2026-02-20T12:00:00Z" {
		t.Errorf("cancelledAt = %v, want the earliest 2026-02-20T12:00:00Z", s.CancelledAt)
	}
}

func TestPostSubscriptionCancellationRejectsCancelledAtBeforeStart(t *testing.T) {
	h := newTestHandler(t)
	userID := createUserID(t, h)
	subID := createSubscriptionID(t, h, userID, "Streaming Premium")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions/"+subID+"/cancellation",
		strings.NewReader(`{"cancelledAt":"2026-01-09T12:00:00Z"}`)))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	var got struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.Error == "" {
		t.Errorf("body = %s, want {\"error\":...}", rec.Body.String())
	}

	s := listSubscriptionsFor(t, h, userID)[0]
	if s.CancelledAt != nil {
		t.Errorf("cancelledAt = %q after rejected cancellation, want null", *s.CancelledAt)
	}
	if s.Phase != "active" {
		t.Errorf("phase = %q after rejected cancellation, want active", s.Phase)
	}
}

func listSubscriptionsFor(t *testing.T, h http.Handler, userID string) []subscriptionBody {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/"+userID+"/subscriptions", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var list []subscriptionBody
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("GET body is not a JSON array: %v; body = %s", err, rec.Body.String())
	}
	return list
}

func TestPostSubscriptionCancellationReturnsNotFoundForUnownedSubscription(t *testing.T) {
	const body = `{"cancelledAt":"2026-02-20T12:00:00Z"}`

	tests := []struct {
		name   string
		userID func(t *testing.T, h http.Handler) string
		subID  func(t *testing.T, h http.Handler) string
	}{
		{
			name:   "unknown user",
			userID: func(*testing.T, http.Handler) string { return missingUserID },
			subID: func(t *testing.T, h http.Handler) string {
				return createSubscriptionID(t, h, createUserID(t, h), "Streaming Premium")
			},
		},
		{
			name:   "unknown subscription",
			userID: func(t *testing.T, h http.Handler) string { return createUserID(t, h) },
			subID:  func(*testing.T, http.Handler) string { return missingUserID },
		},
		{
			name:   "subscription owned by another user",
			userID: func(t *testing.T, h http.Handler) string { return createUserID(t, h) },
			subID: func(t *testing.T, h http.Handler) string {
				return createSubscriptionID(t, h, createUserID(t, h), "Streaming Premium")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(t)
			userID := tt.userID(t, h)
			subID := tt.subID(t, h)

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions/"+subID+"/cancellation", strings.NewReader(body)))

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
			}
		})
	}
}

func TestPostSubscriptionsCreatesSubscription(t *testing.T) {
	h := newTestHandler(t)
	userID := createUserID(t, h)

	req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/subscriptions", strings.NewReader(
		`{`+streamingPlanBody+`,"initialAmount":30,"startDate":"2026-01-10T12:00:00Z"}`))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got subscriptionBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v; body = %s", err, rec.Body.String())
	}
	if _, err := uuid.Parse(got.ID); err != nil {
		t.Errorf("id = %q, want UUID", got.ID)
	}
	if got.Plan.Name != "Streaming Premium" || got.Plan.Vendor != "StreamCo" ||
		got.Plan.Description == nil || *got.Plan.Description != "Two screens" ||
		got.Plan.BillingFrequency != "monthly" {
		t.Errorf("plan = %+v, want Streaming Premium/StreamCo/Two screens/monthly", got.Plan)
	}
	if got.PaymentMethodID != nil {
		t.Errorf("paymentMethodID = %q, want null", *got.PaymentMethodID)
	}
	if got.StartDate != "2026-01-10T12:00:00Z" {
		t.Errorf("startDate = %q, want 2026-01-10T12:00:00Z", got.StartDate)
	}
	if got.TrialEnd != nil {
		t.Errorf("trialEnd = %q, want null", *got.TrialEnd)
	}
	if got.CancelledAt != nil {
		t.Errorf("cancelledAt = %q, want null", *got.CancelledAt)
	}
	if len(got.Intervals) != 1 || got.Intervals[0].From != "2026-01-10T12:00:00Z" || got.Intervals[0].Amount != 30 {
		t.Errorf("intervals = %+v, want [{2026-01-10T12:00:00Z 30}]", got.Intervals)
	}
}
