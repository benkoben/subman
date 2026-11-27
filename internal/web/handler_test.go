package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"subman/internal/app"
	"subman/internal/payment"
	"subman/internal/plan"
	"subman/internal/repository/memory"
	"subman/internal/subscription"
	"subman/internal/user"
	"subman/internal/web"
)

var testNow = time.Date(2026, time.November, 15, 12, 0, 0, 0, time.UTC)

func newTestHandler(t *testing.T) (http.Handler, *app.Service) {
	t.Helper()
	users := memory.NewRepository(func(u user.User) string { return u.ID().String() })
	cards := memory.NewRepository(func(c *payment.Card) string { return c.ID().String() })
	subs := memory.NewRepository(func(s *subscription.Subscription) string { return s.ID().String() })
	svc := app.NewService(users, cards, subs)
	return web.NewServer(svc, web.WithClock(func() time.Time { return testNow })), svc
}

func request(t *testing.T, handler http.Handler, method, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if form == nil {
		body = strings.NewReader("")
	} else {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body.String())
	}
}

func requireBodyContains(t *testing.T, rec *httptest.ResponseRecorder, values ...string) {
	t.Helper()
	body := rec.Body.String()
	for _, value := range values {
		if !strings.Contains(body, value) {
			t.Errorf("body does not contain %q:\n%s", value, body)
		}
	}
}

func TestUsersPageAndCreateUser(t *testing.T) {
	handler, svc := newTestHandler(t)

	rec := request(t, handler, http.MethodGet, "/", nil)
	requireStatus(t, rec, http.StatusOK)
	requireBodyContains(t, rec, "No Users yet", `method="post" action="/profiles"`)

	rec = request(t, handler, http.MethodPost, "/profiles", url.Values{
		"name":  {"Erika Lindqvist"},
		"email": {"not-an-email"},
	})
	requireStatus(t, rec, http.StatusBadRequest)
	requireBodyContains(t, rec, "user email must contain @", `value="Erika Lindqvist"`)

	rec = request(t, handler, http.MethodPost, "/profiles", url.Values{
		"name":  {"Erika Lindqvist"},
		"email": {"erika@example.com"},
	})
	requireStatus(t, rec, http.StatusSeeOther)
	profile, err := svc.Profile(context.Background(), mustUserID(t, svc))
	if err != nil {
		t.Fatalf("Profile() error = %v", err)
	}
	wantLocation := "/profiles/" + profile.User.ID().String()
	if location := rec.Header().Get("Location"); location != wantLocation {
		t.Errorf("Location = %q, want %q", location, wantLocation)
	}

	rec = request(t, handler, http.MethodGet, wantLocation, nil)
	requireStatus(t, rec, http.StatusOK)
	requireBodyContains(t, rec, "Erika Lindqvist", "erika@example.com", "No Subscriptions yet", "Your Wallet is empty")
}

func TestCompleteWebWorkflow(t *testing.T) {
	handler, svc := newTestHandler(t)
	ctx := context.Background()
	createdUser, err := svc.CreateUser(ctx, "Erika Lindqvist", "erika@example.com")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	userID := createdUser.ID().String()
	profilePath := "/profiles/" + userID

	rec := request(t, handler, http.MethodGet, profilePath+"/cards/new", nil)
	requireStatus(t, rec, http.StatusOK)
	requireBodyContains(t, rec, "Add a Card", `value="2026"`)

	rec = request(t, handler, http.MethodPost, profilePath+"/cards/new", url.Values{
		"name":        {"Everyday debit"},
		"cardType":    {"debit"},
		"expiryYear":  {"2028"},
		"expiryMonth": {"8"},
	})
	requireStatus(t, rec, http.StatusSeeOther)
	profile, err := svc.Profile(ctx, createdUser.ID())
	if err != nil || len(profile.Cards) != 1 {
		t.Fatalf("Profile() cards = %d, error = %v; want one card", len(profile.Cards), err)
	}
	cardID := profile.Cards[0].ID().String()

	rec = request(t, handler, http.MethodGet, profilePath+"/subscriptions/new", nil)
	requireStatus(t, rec, http.StatusOK)
	requireBodyContains(t, rec, "Everyday debit", `value="2026-11-15T12:00:00"`)

	rec = request(t, handler, http.MethodPost, profilePath+"/subscriptions/new", url.Values{
		"planName":         {"Premium"},
		"vendor":           {"Example Streaming"},
		"description":      {"Family plan"},
		"billingFrequency": {"monthly"},
		"initialAmount":    {"120"},
		"startDate":        {"2026-10-01T09:00:00"},
		"paymentMethodID":  {cardID},
	})
	requireStatus(t, rec, http.StatusSeeOther)
	profile, err = svc.Profile(ctx, createdUser.ID())
	if err != nil || len(profile.Subscriptions) != 1 {
		t.Fatalf("Profile() subscriptions = %d, error = %v; want one", len(profile.Subscriptions), err)
	}
	subscriptionID := profile.Subscriptions[0].ID().String()
	subscriptionPath := profilePath + "/subscriptions/" + subscriptionID

	rec = request(t, handler, http.MethodGet, subscriptionPath, nil)
	requireStatus(t, rec, http.StatusOK)
	requireBodyContains(t, rec, "Premium", "Example Streaming", "Everyday debit", "120 SEK", "240 SEK", "Intro Offer")

	rec = request(t, handler, http.MethodGet, subscriptionPath+"/price-intervals", nil)
	requireStatus(t, rec, http.StatusOK)
	requireBodyContains(t, rec, "Add a Price Interval", `value="2026-11-15T12:00:00"`)

	intervalForm := url.Values{"from": {"2026-12-01T09:00:00"}, "amount": {"150"}}
	rec = request(t, handler, http.MethodPost, subscriptionPath+"/price-intervals", intervalForm)
	requireStatus(t, rec, http.StatusSeeOther)
	rec = request(t, handler, http.MethodPost, subscriptionPath+"/price-intervals", intervalForm)
	requireStatus(t, rec, http.StatusBadRequest)
	requireBodyContains(t, rec, "price interval already exists")

	rec = request(t, handler, http.MethodGet, subscriptionPath, nil)
	requireStatus(t, rec, http.StatusOK)
	requireBodyContains(t, rec, "150 SEK", "Current amount", "120 SEK per month")

	rec = request(t, handler, http.MethodGet, subscriptionPath+"/cancellation", nil)
	requireStatus(t, rec, http.StatusOK)
	requireBodyContains(t, rec, "does not contact Example Streaming", "Cancellation is terminal")
	rec = request(t, handler, http.MethodPost, subscriptionPath+"/cancellation", url.Values{
		"cancelledAt": {"2026-11-15T12:00:00"},
	})
	requireStatus(t, rec, http.StatusSeeOther)

	rec = request(t, handler, http.MethodGet, profilePath+"/cards/"+cardID+"/removal", nil)
	requireStatus(t, rec, http.StatusOK)
	requireBodyContains(t, rec, "does not delete the Card", `value="2026-11-15T12:00:00"`)
	rec = request(t, handler, http.MethodPost, profilePath+"/cards/"+cardID+"/removal", url.Values{
		"removedAt": {"2026-11-15T12:00:00"},
	})
	requireStatus(t, rec, http.StatusSeeOther)

	rec = request(t, handler, http.MethodGet, profilePath, nil)
	requireStatus(t, rec, http.StatusOK)
	requireBodyContains(t, rec, "Cancelled", "Removed", "0 SEK")
}

func TestProfileSummarySeparatesFrequenciesAndTrials(t *testing.T) {
	handler, svc := newTestHandler(t)
	ctx := context.Background()
	createdUser, err := svc.CreateUser(ctx, "Erika Lindqvist", "erika@example.com")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	start := time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC)
	for _, input := range []app.SubscriptionInput{
		{PlanName: "Monthly", PlanVendor: "Vendor", BillingFrequency: plan.Monthly, InitialAmount: 100, StartDate: start},
		{PlanName: "Annual", PlanVendor: "Vendor", BillingFrequency: plan.Annually, InitialAmount: 1200, StartDate: start},
	} {
		if _, err := svc.CreateSubscription(ctx, createdUser.ID(), input); err != nil {
			t.Fatalf("CreateSubscription() error = %v", err)
		}
	}
	trialDays := 30
	if _, err := svc.CreateSubscription(ctx, createdUser.ID(), app.SubscriptionInput{
		PlanName: "Trial", PlanVendor: "Vendor", BillingFrequency: plan.Monthly,
		InitialAmount: 50, StartDate: testNow.AddDate(0, 0, -1), TrialDays: &trialDays,
	}); err != nil {
		t.Fatalf("CreateSubscription() error = %v", err)
	}

	rec := request(t, handler, http.MethodGet, "/profiles/"+createdUser.ID().String(), nil)
	requireStatus(t, rec, http.StatusOK)
	requireBodyContains(t, rec, "100 SEK", "1\u202f200 SEK", "<strong>2</strong> Active", "<strong>1</strong> Trialing")
}

func TestStaticAssetsNotFoundAndInvalidIDs(t *testing.T) {
	handler, _ := newTestHandler(t)

	for _, asset := range []string{"/static/app.css", "/static/theme.js"} {
		rec := request(t, handler, http.MethodGet, asset, nil)
		requireStatus(t, rec, http.StatusOK)
		if !strings.Contains(rec.Header().Get("Content-Type"), "text/") && !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
			t.Errorf("%s Content-Type = %q, want text or JavaScript", asset, rec.Header().Get("Content-Type"))
		}
	}

	rec := request(t, handler, http.MethodGet, "/does-not-exist", nil)
	requireStatus(t, rec, http.StatusNotFound)
	requireBodyContains(t, rec, "The requested page could not be found")

	rec = request(t, handler, http.MethodGet, "/profiles/not-a-uuid", nil)
	requireStatus(t, rec, http.StatusBadRequest)
}

func mustUserID(t *testing.T, svc *app.Service) uuid.UUID {
	t.Helper()
	users, err := svc.ListUsers(context.Background())
	if err != nil || len(users) != 1 {
		t.Fatalf("ListUsers() returned %d Users, error = %v; want one", len(users), err)
	}
	return users[0].ID()
}
