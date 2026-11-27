package web

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"subman/internal/app"
	"subman/internal/payment"
	"subman/internal/plan"
	"subman/internal/subscription"
	"subman/internal/user"
)

type baseData struct {
	Title   string
	Content string
	User    *userView
	Error   string
}

func (b baseData) pageBase() baseData {
	return b
}

type userView struct {
	ID    string
	Name  string
	Email string
}

type usersPage struct {
	baseData
	Users []userView
	Form  url.Values
}

type summaryView struct {
	MonthlyRecurring int64
	AnnualRecurring  int64
	TotalPaid        int64
	Active           int
	Trialing         int
	Cancelled        int
}

type intervalView struct {
	From    string
	FromRFC string
	Amount  int64
}

type subscriptionView struct {
	ID                 string
	PlanName           string
	Vendor             string
	Description        *string
	BillingFrequency   plan.BillingFrequency
	FrequencyLabel     string
	Phase              subscription.Phase
	PhaseLabel         string
	DisplayAmount      int64
	DisplayAmountLabel string
	ShowAmount         bool
	TotalPaid          int64
	StartDate          string
	StartDateRFC       string
	TrialEnd           string
	TrialEndRFC        string
	CancelledAt        string
	CancelledAtRFC     string
	PaymentMethodName  string
	Intervals          []intervalView
	CanCancel          bool
}

type cardView struct {
	ID          string
	Name        string
	CardType    payment.CardType
	TypeLabel   string
	Expiry      string
	Status      string
	StatusClass string
	RemovedAt   string
	CanRemove   bool
}

type profilePage struct {
	baseData
	Summary       summaryView
	Subscriptions []subscriptionView
	Cards         []cardView
}

type subscriptionPage struct {
	baseData
	Subscription subscriptionView
}

type cardFormPage struct {
	baseData
	Form url.Values
}

type subscriptionFormPage struct {
	baseData
	Form  url.Values
	Cards []cardView
}

type intervalPage struct {
	baseData
	Subscription subscriptionView
	Form         url.Values
}

type confirmationPage struct {
	baseData
	Form         url.Values
	Subscription *subscriptionView
	Card         *cardView
}

func newBase(title, content string, u user.User) baseData {
	view := newUserView(u)
	return baseData{Title: title, Content: content, User: &view}
}

func newUserView(u user.User) userView {
	return userView{ID: u.ID().String(), Name: u.Name(), Email: u.Email()}
}

func newUsersPage(users []user.User, form url.Values) usersPage {
	views := make([]userView, 0, len(users))
	for _, u := range users {
		views = append(views, newUserView(u))
	}
	sort.Slice(views, func(i, j int) bool {
		if views[i].Name == views[j].Name {
			return views[i].Email < views[j].Email
		}
		return views[i].Name < views[j].Name
	})
	return usersPage{
		baseData: baseData{Title: "Your profiles", Content: "users"},
		Users:    views,
		Form:     form,
	}
}

func newProfilePage(p app.Profile, now time.Time) (profilePage, error) {
	cards := make([]cardView, 0, len(p.Cards))
	cardNames := make(map[string]string, len(p.Cards))
	for _, card := range p.Cards {
		view := newCardView(card, now)
		cards = append(cards, view)
		cardNames[view.ID] = view.Name
	}
	sort.Slice(cards, func(i, j int) bool { return cards[i].Name < cards[j].Name })

	page := profilePage{
		baseData:      newBase(p.User.Name(), "profile", p.User),
		Subscriptions: make([]subscriptionView, 0, len(p.Subscriptions)),
		Cards:         cards,
	}
	for _, sub := range p.Subscriptions {
		view, err := newSubscriptionView(sub, now, cardNames)
		if err != nil {
			return profilePage{}, err
		}
		page.Subscriptions = append(page.Subscriptions, view)
		page.Summary.TotalPaid += view.TotalPaid

		switch view.Phase {
		case subscription.PhaseActive:
			page.Summary.Active++
			switch view.BillingFrequency {
			case plan.Monthly:
				page.Summary.MonthlyRecurring += view.DisplayAmount
			case plan.Annually:
				page.Summary.AnnualRecurring += view.DisplayAmount
			}
		case subscription.PhaseTrialing:
			page.Summary.Trialing++
		case subscription.PhaseCancelled:
			page.Summary.Cancelled++
		}
	}
	sort.Slice(page.Subscriptions, func(i, j int) bool {
		if page.Subscriptions[i].PlanName == page.Subscriptions[j].PlanName {
			return page.Subscriptions[i].Vendor < page.Subscriptions[j].Vendor
		}
		return page.Subscriptions[i].PlanName < page.Subscriptions[j].PlanName
	})

	return page, nil
}

func newSubscriptionView(sub *subscription.Subscription, now time.Time, cardNames map[string]string) (subscriptionView, error) {
	p := sub.Plan()
	phase := sub.PhaseAt(now)
	totalPaid, err := sub.TotalPaidAt(now)
	if err != nil {
		return subscriptionView{}, err
	}

	view := subscriptionView{
		ID:                 sub.ID().String(),
		PlanName:           p.Name,
		Vendor:             p.Vendor,
		Description:        p.Description,
		BillingFrequency:   p.BillingFrequency,
		Phase:              phase,
		PhaseLabel:         phaseLabel(phase),
		TotalPaid:          totalPaid.Amount(),
		StartDate:          formatTime(sub.StartDate()),
		StartDateRFC:       sub.StartDate().UTC().Format(time.RFC3339),
		Intervals:          make([]intervalView, 0, len(sub.Intervals())),
		CanCancel:          phase != subscription.PhaseCancelled,
		ShowAmount:         phase != subscription.PhaseCancelled,
		DisplayAmountLabel: "Current amount",
	}
	if p.BillingFrequency == plan.Monthly {
		view.FrequencyLabel = "month"
	} else {
		view.FrequencyLabel = "year"
	}

	if amount, ok := sub.CurrentAmountAt(now); ok {
		view.DisplayAmount = amount.Amount()
	} else {
		intervals := sub.Intervals()
		if len(intervals) > 0 {
			openingAmount := intervals[0].Amount()
			view.DisplayAmount = openingAmount.Amount()
			view.DisplayAmountLabel = "Opening amount"
		}
	}
	if sub.PaymentMethodID() != nil {
		view.PaymentMethodName = cardNames[sub.PaymentMethodID().String()]
		if view.PaymentMethodName == "" {
			view.PaymentMethodName = "Unknown Card"
		}
	}
	if sub.TrialEnd() != nil {
		view.TrialEnd = formatTime(*sub.TrialEnd())
		view.TrialEndRFC = sub.TrialEnd().UTC().Format(time.RFC3339)
	}
	if sub.CancelledAt() != nil {
		view.CancelledAt = formatTime(*sub.CancelledAt())
		view.CancelledAtRFC = sub.CancelledAt().UTC().Format(time.RFC3339)
	}
	for _, interval := range sub.Intervals() {
		amount := interval.Amount()
		view.Intervals = append(view.Intervals, intervalView{
			From:    formatTime(interval.From()),
			FromRFC: interval.From().UTC().Format(time.RFC3339),
			Amount:  amount.Amount(),
		})
	}

	return view, nil
}

func newCardView(card *payment.Card, now time.Time) cardView {
	year, month := card.Expiry()
	view := cardView{
		ID:          card.ID().String(),
		Name:        card.Name(),
		CardType:    card.CardType(),
		Expiry:      fmt.Sprintf("%s %d", month.String(), year),
		Status:      "Active",
		StatusClass: "status-active",
		CanRemove:   card.RemovedAt() == nil,
	}
	if card.CardType() == payment.Debit {
		view.TypeLabel = "Debit"
	} else {
		view.TypeLabel = "Credit"
	}
	if card.ExpiredAt(now) {
		view.Status = "Expired"
		view.StatusClass = "status-expired"
	}
	if card.RemovedAt() != nil {
		view.Status = "Removed"
		view.StatusClass = "status-removed"
		view.RemovedAt = formatTime(*card.RemovedAt())
	}
	return view
}

func phaseLabel(phase subscription.Phase) string {
	switch phase {
	case subscription.PhaseTrialing:
		return "Trialing"
	case subscription.PhaseCancelled:
		return "Cancelled"
	default:
		return "Active"
	}
}

func formatTime(t time.Time) string {
	return t.UTC().Format("2 Jan 2006, 15:04 UTC")
}

func inputTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05")
}

func formatSEK(amount int64) string {
	digits := strconv.FormatInt(amount, 10)
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign = "-"
		digits = strings.TrimPrefix(digits, "-")
	}
	for i := len(digits) - 3; i > 0; i -= 3 {
		digits = digits[:i] + "\u202f" + digits[i:]
	}
	return sign + digits + " SEK"
}

func profileURL(userID string) string {
	return "/profiles/" + userID
}

func newCardURL(userID string) string {
	return profileURL(userID) + "/cards/new"
}

func newSubscriptionURL(userID string) string {
	return profileURL(userID) + "/subscriptions/new"
}

func subscriptionURL(userID, subscriptionID string) string {
	return profileURL(userID) + "/subscriptions/" + subscriptionID
}

func cancellationURL(userID, subscriptionID string) string {
	return subscriptionURL(userID, subscriptionID) + "/cancellation"
}

func priceIntervalURL(userID, subscriptionID string) string {
	return subscriptionURL(userID, subscriptionID) + "/price-intervals"
}

func cardRemovalURL(userID, cardID string) string {
	return profileURL(userID) + "/cards/" + cardID + "/removal"
}
