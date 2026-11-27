package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"subman/internal/api"
	"subman/internal/app"
	"subman/internal/repository/mongodb"
	"subman/internal/web"
)

const (
	defaultMongoURI = "mongodb://localhost:27017"
	defaultDatabase = "subman"
	defaultHTTPAddr = ":8080"

	startupTimeout  = 10 * time.Second
	shutdownTimeout = 10 * time.Second
)

func main() {
	if err := run(context.Background(), os.Getenv, os.Stderr, net.Listen); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(
	ctx context.Context,
	getenv func(string) string,
	stderr io.Writer,
	listen func(network, address string) (net.Listener, error),
) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := log.New(stderr, "", log.LstdFlags)

	startupCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	client, err := mongodb.Connect(startupCtx, envOrDefault(getenv, "MONGODB_URI", defaultMongoURI))
	cancel()
	if err != nil {
		return err
	}
	defer disconnect(client, logger)

	database := envOrDefault(getenv, "MONGODB_DATABASE", defaultDatabase)
	svc := app.NewService(
		mongodb.NewUserRepository(mongodb.Collection(client, database, "users")),
		mongodb.NewCardRepository(mongodb.Collection(client, database, "cards")),
		mongodb.NewSubscriptionRepository(mongodb.Collection(client, database, "subscriptions")),
	)

	addr := envOrDefault(getenv, "HTTP_ADDR", defaultHTTPAddr)
	listener, err := listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen http: %w", err)
	}

	apiHandler := api.NewServer(svc)
	handler := http.NewServeMux()
	handler.Handle("/healthz", apiHandler)
	handler.Handle("/users", apiHandler)
	handler.Handle("/users/", apiHandler)
	handler.Handle("/", web.NewServer(svc))

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ErrorLog:          logger,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(listener)
	}()
	logger.Printf("subman listening on %s", listener.Addr())

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve http: %w", err)
	case <-ctx.Done():
	}

	logger.Print("shutting down")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()
	shutdownErr := srv.Shutdown(shutdownCtx)
	serveErrValue := <-serveErr

	if shutdownErr != nil {
		return fmt.Errorf("shutdown http server: %w", shutdownErr)
	}
	if serveErrValue != nil && !errors.Is(serveErrValue, http.ErrServerClosed) {
		return fmt.Errorf("serve http: %w", serveErrValue)
	}

	return nil
}

func disconnect(client *mongo.Client, logger *log.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := client.Disconnect(ctx); err != nil {
		logger.Printf("disconnect mongodb: %v", err)
	}
}

func envOrDefault(getenv func(string) string, key, fallback string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return fallback
}
