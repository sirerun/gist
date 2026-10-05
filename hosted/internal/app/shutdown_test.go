package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func shutdownTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	// Pool construction is lazy: these lifecycle tests never acquire a connection.
	pool, err := pgxpool.New(context.Background(), "postgres://fixture@localhost/fixture?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestShutdownDeadlineBoundsJanitorWait(t *testing.T) {
	janitorDone := make(chan struct{})
	janitorCancelled := make(chan struct{})
	a := &App{pool: shutdownTestPool(t), server: &http.Server{}, janitorDone: janitorDone, janitorCancel: func() { close(janitorCancelled) }}
	defer close(janitorDone)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	returned := make(chan error, 1)
	go func() { returned <- a.Shutdown(ctx) }()
	select {
	case err := <-returned:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("shutdown error = %v, want deadline exceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown ignored caller deadline while janitor was blocked")
	}
	select {
	case <-janitorCancelled:
	default:
		t.Fatal("shutdown did not cancel the janitor")
	}
}

func TestShutdownRetainsHTTPDrainErrorForLaterCallers(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))
	defer s.Close()
	defer close(release)
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		response, err := s.Client().Get(s.URL)
		if err == nil {
			_ = response.Body.Close() // No response payload is written or persisted.
		}
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("HTTP request did not enter handler")
	}
	a := &App{pool: shutdownTestPool(t), server: s.Config}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if err := a.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("first shutdown error = %v, want deadline exceeded", err)
	}
	later, cancelLater := context.WithTimeout(context.Background(), time.Second)
	defer cancelLater()
	if err := a.Shutdown(later); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("later shutdown error = %v, want retained deadline exceeded", err)
	}
}
