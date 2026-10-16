package db

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOpenRejectsBadDSN(t *testing.T) {
	if _, err := Open(context.Background(), "postgres://%zz"); err == nil {
		t.Fatal("expected an error for a malformed DSN")
	}
}

func TestOpenIsLazy(t *testing.T) {
	// Nothing listens on this port: Open must still succeed, only Ping fails.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := Open(ctx, "postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err == nil {
		t.Fatal("expected Ping to fail without a server")
	}
}

// Integration test: runs only when TEST_DATABASE_URL points to a PostgreSQL 18 server.
func TestPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	var version string
	if err := pool.QueryRow(ctx, "SHOW server_version").Scan(&version); err != nil {
		t.Fatalf("server_version: %v", err)
	}
	if !strings.HasPrefix(version, "18.") {
		t.Fatalf("server_version = %q, want 18.x", version)
	}

	// Kazakh letters must round-trip and case-fold (address matching depends on it).
	var lower string
	if err := pool.QueryRow(ctx, "SELECT lower($1)", "ҚАРАҒАНДЫ ӘЛІ").Scan(&lower); err != nil {
		t.Fatalf("lower: %v", err)
	}
	if lower != "қарағанды әлі" {
		t.Fatalf("lower = %q", lower)
	}
}
