package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestRunServesAndShutsDown(t *testing.T) {
	mongoURI := os.Getenv("SUBMAN_TEST_MONGODB_URI")
	if mongoURI == "" {
		t.Skip("set SUBMAN_TEST_MONGODB_URI to run the real-MongoDB process integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	database := "subman_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	cleanupClient, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatalf("connect cleanup client: %v", err)
	}
	defer func() {
		cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelCleanup()
		if err := cleanupClient.Database(database).Drop(cleanupCtx); err != nil {
			t.Errorf("drop test database %s: %v", database, err)
		}
		if err := cleanupClient.Disconnect(cleanupCtx); err != nil {
			t.Errorf("disconnect cleanup client: %v", err)
		}
	}()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pre-bind listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	getenv := func(key string) string {
		switch key {
		case "MONGODB_URI":
			return mongoURI
		case "MONGODB_DATABASE":
			return database
		case "HTTP_ADDR":
			return listener.Addr().String()
		default:
			return ""
		}
	}
	listen := func(_, _ string) (net.Listener, error) {
		return listener, nil
	}

	var logs lockedBuffer
	runErr := make(chan error, 1)
	stopped := false
	stop := func() error {
		if stopped {
			return nil
		}
		stopped = true
		cancel()
		select {
		case err := <-runErr:
			return err
		case <-time.After(shutdownTimeout + 5*time.Second):
			return errors.New("run did not stop within the shutdown deadline")
		}
	}
	defer func() {
		if err := stop(); err != nil {
			t.Errorf("stop run: %v; logs:\n%s", err, logs.String())
		}
	}()

	go func() {
		runErr <- run(ctx, getenv, &logs, listen)
	}()

	baseURL := "http://" + listener.Addr().String()
	if err := waitForReady(ctx, baseURL+"/healthz"); err != nil {
		t.Fatalf("wait for readiness: %v; logs:\n%s", err, logs.String())
	}

	client := &http.Client{Timeout: 2 * time.Second}
	status, body, err := requestJSON(ctx, client, http.MethodPost, baseURL+"/users",
		`{"name":"Erika Lindqvist","email":"erika@example.com"}`)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if status != http.StatusCreated {
		t.Fatalf("create user status = %d, want %d; body = %s", status, http.StatusCreated, body)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &created); err != nil || created.ID == "" {
		t.Fatalf("create user body = %s, want an ID; decode error = %v", body, err)
	}

	status, body, err = requestJSON(ctx, client, http.MethodGet, baseURL+"/users", "")
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("list users status = %d, want %d; body = %s", status, http.StatusOK, body)
	}
	var users []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &users); err != nil {
		t.Fatalf("list users body = %s: %v", body, err)
	}
	if len(users) != 1 || users[0].ID != created.ID {
		t.Fatalf("listed users = %s, want only created user %s", body, created.ID)
	}

	if err := stop(); err != nil {
		t.Fatalf("run returned an error: %v; logs:\n%s", err, logs.String())
	}
}

func waitForReady(ctx context.Context, endpoint string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client := &http.Client{Timeout: time.Second}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		status, _, err := requestJSON(ctx, client, http.MethodGet, endpoint, "")
		if err == nil && status == http.StatusOK {
			return nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("health status = %d, want %d", status, http.StatusOK)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("readiness deadline: %w; last result: %v", ctx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}

func requestJSON(
	ctx context.Context,
	client *http.Client,
	method, endpoint, body string,
) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, responseBody, nil
}
