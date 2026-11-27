// Package web serves Subman's server-rendered HTML interface.
package web

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"time"

	"github.com/google/uuid"

	"subman/internal/app"
	"subman/internal/payment"
	"subman/internal/subscription"
	"subman/internal/user"
)

//go:embed templates/*.html static/*
var assets embed.FS

// Service is the application boundary used by the HTML interface. Keeping the
// boundary here lets the web module evolve independently of HTTP API handlers.
type Service interface {
	CreateUser(ctx context.Context, name, email string) (user.User, error)
	ListUsers(ctx context.Context) ([]user.User, error)
	Profile(ctx context.Context, ownerID uuid.UUID) (app.Profile, error)
	CreateCard(ctx context.Context, ownerID uuid.UUID, name string, cardType payment.CardType, expiryYear int, expiryMonth time.Month) (*payment.Card, error)
	CreateSubscription(ctx context.Context, ownerID uuid.UUID, in app.SubscriptionInput) (*subscription.Subscription, error)
	AddPriceInterval(ctx context.Context, ownerID, subscriptionID uuid.UUID, from time.Time, amount int64) (*subscription.Subscription, error)
	CancelSubscription(ctx context.Context, ownerID, subscriptionID uuid.UUID, at time.Time) (*subscription.Subscription, error)
	RemoveCard(ctx context.Context, ownerID, cardID uuid.UUID, at time.Time) (*payment.Card, error)
}

type options struct {
	clock func() time.Time
}

// Option configures NewServer.
type Option func(*options)

// WithClock sets the source of current time for derived state and form values.
func WithClock(clock func() time.Time) Option {
	return func(o *options) {
		o.clock = clock
	}
}

type server struct {
	service   Service
	clock     func() time.Time
	mux       *http.ServeMux
	templates map[string]*template.Template
}

// NewServer constructs the server-rendered web interface.
func NewServer(service Service, opts ...Option) http.Handler {
	cfg := options{clock: time.Now}
	for _, opt := range opts {
		opt(&cfg)
	}

	s := &server{
		service:   service,
		clock:     cfg.clock,
		mux:       http.NewServeMux(),
		templates: parseTemplates(),
	}

	s.addRoutes()
	return s.mux
}

func parseTemplates() map[string]*template.Template {
	base := template.Must(template.New("web").Funcs(template.FuncMap{
		"sek":           formatSEK,
		"formatTime":    formatTime,
		"inputTime":     inputTime,
		"profileURL":    profileURL,
		"newCardURL":    newCardURL,
		"newSubURL":     newSubscriptionURL,
		"subURL":        subscriptionURL,
		"cancelURL":     cancellationURL,
		"intervalURL":   priceIntervalURL,
		"removeCardURL": cardRemovalURL,
	}).ParseFS(assets, "templates/base.html"))

	files := map[string]string{
		"users":             "templates/users.html",
		"profile":           "templates/profile.html",
		"subscription":      "templates/subscription.html",
		"card-form":         "templates/card_form.html",
		"subscription-form": "templates/subscription_form.html",
		"interval-form":     "templates/interval_form.html",
		"cancellation":      "templates/confirmations.html",
		"card-removal":      "templates/card_removal.html",
		"error":             "templates/error.html",
	}
	parsed := make(map[string]*template.Template, len(files))
	for name, file := range files {
		clone, err := base.Clone()
		if err != nil {
			panic(err)
		}
		parsed[name] = template.Must(clone.ParseFS(assets, file))
	}
	return parsed
}

func (s *server) addRoutes() {
	staticFiles, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err)
	}
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))))

	s.mux.HandleFunc("GET /{$}", s.handleUsers)
	s.mux.HandleFunc("POST /profiles", s.handleCreateUser)
	s.mux.HandleFunc("GET /profiles/{userID}", s.handleProfile)
	s.mux.HandleFunc("GET /profiles/{userID}/cards/new", s.handleNewCard)
	s.mux.HandleFunc("POST /profiles/{userID}/cards/new", s.handleCreateCard)
	s.mux.HandleFunc("GET /profiles/{userID}/cards/{cardID}/removal", s.handleRemoveCard)
	s.mux.HandleFunc("POST /profiles/{userID}/cards/{cardID}/removal", s.handleRemoveCard)
	s.mux.HandleFunc("GET /profiles/{userID}/subscriptions/new", s.handleNewSubscription)
	s.mux.HandleFunc("POST /profiles/{userID}/subscriptions/new", s.handleCreateSubscription)
	s.mux.HandleFunc("GET /profiles/{userID}/subscriptions/{subscriptionID}", s.handleSubscription)
	s.mux.HandleFunc("GET /profiles/{userID}/subscriptions/{subscriptionID}/price-intervals", s.handlePriceInterval)
	s.mux.HandleFunc("POST /profiles/{userID}/subscriptions/{subscriptionID}/price-intervals", s.handlePriceInterval)
	s.mux.HandleFunc("GET /profiles/{userID}/subscriptions/{subscriptionID}/cancellation", s.handleCancellation)
	s.mux.HandleFunc("POST /profiles/{userID}/subscriptions/{subscriptionID}/cancellation", s.handleCancellation)
	s.mux.HandleFunc("GET /", s.handleNotFound)
}
