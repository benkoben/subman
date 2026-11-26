package api

import (
	"net/http"
	"time"

	"subman/internal/app"
)

type options struct {
	clock func() time.Time
}

// Option configures NewServer.
type Option func(*options)

// WithClock sets the source of the current time used to evaluate derived
// state in responses, such as a Subscription's phase and total paid. Each
// response reads the clock once.
func WithClock(clock func() time.Time) Option {
	return func(o *options) {
		o.clock = clock
	}
}

// NewServer constructs the HTTP API over the given Service. Without options
// it evaluates derived state at time.Now.
func NewServer(svc *app.Service, opts ...Option) http.Handler {
	cfg := options{clock: time.Now}
	for _, opt := range opts {
		opt(&cfg)
	}

	mux := http.NewServeMux()
	addRoutes(mux, svc, cfg.clock)

	var handler http.Handler = mux
	return handler
}
