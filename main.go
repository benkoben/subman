package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"subman/internal/api"
	"subman/internal/app"
	"subman/internal/repository/mongodb"
)

const (
	defaultMongoURI = "mongodb://localhost:27017"
	defaultDatabase = "subman"
	defaultHTTPAddr = ":8080"

	startupTimeout  = 10 * time.Second
	shutdownTimeout = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startupCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	client, err := mongodb.Connect(startupCtx, getenv("MONGODB_URI", defaultMongoURI))
	cancel()
	if err != nil {
		return err
	}
	defer disconnect(client)

	database := getenv("MONGODB_DATABASE", defaultDatabase)
	svc := app.NewService(
		mongodb.NewUserRepository(mongodb.Collection(client, database, "users")),
		mongodb.NewCardRepository(mongodb.Collection(client, database, "cards")),
		mongodb.NewSubscriptionRepository(mongodb.Collection(client, database, "subscriptions")),
	)

	addr := getenv("HTTP_ADDR", defaultHTTPAddr)
	srv := &http.Server{
		Addr:              addr,
		Handler:           api.NewHandler(svc),
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.ListenAndServe()
	}()
	log.Printf("subman listening on %s", addr)

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve http: %w", err)
	case <-ctx.Done():
	}

	log.Print("shutting down")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	return nil
}

func disconnect(client *mongo.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := client.Disconnect(ctx); err != nil {
		log.Printf("disconnect mongodb: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
