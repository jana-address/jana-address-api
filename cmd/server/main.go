// Command server runs the Jana Address HTTP API.
//
// Subcommands:
//
//	serve        start the HTTP server (default); migrates the database and loads KATO in the background
//	migrate      apply the database migrations and seed the KATO snapshot, then exit
//	healthcheck  probe /readyz of a running server; used by the Docker HEALTHCHECK (distroless has no shell)
//	version      print the build version
//
// Environment: HTTP_ADDR, DATABASE_URL, OFFLINE (default true: never call stat.gov.kz),
// KATO_REFRESH_INTERVAL (default 168h), KATO_PAGE_URL.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jana-address/jana-address-api/internal/db"
	"github.com/jana-address/jana-address-api/internal/httpapi"
	"github.com/jana-address/jana-address-api/internal/katosync"
)

// version is set at build time: -ldflags "-X main.version=..."
var version = "dev"

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	var err error
	switch cmd {
	case "serve":
		err = serve()
	case "migrate":
		err = migrate()
	case "healthcheck":
		err = healthcheck()
	case "version":
		fmt.Println(version)
	default:
		err = fmt.Errorf("unknown command %q (want serve, migrate, healthcheck or version)", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func serve() error {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	addr := env("HTTP_ADDR", ":8080")
	dsn := env("DATABASE_URL", "postgres://jana:jana_local_only@localhost:5432/jana?sslmode=disable")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	cfg, err := syncConfig()
	if err != nil {
		return err
	}
	syncer, err := katosync.New(pool, log, cfg)
	if err != nil {
		return err
	}
	go syncer.Run(ctx)

	srv := &http.Server{
		Addr:              addr,
		Handler:           httpapi.New(pool, version, log, httpapi.WithDirectory(func() any { return syncer.Status() })),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", addr, "version", version)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
	}
	return nil
}

func migrate() error {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pool, err := db.Open(ctx, env("DATABASE_URL", "postgres://jana:jana_local_only@localhost:5432/jana?sslmode=disable"))
	if err != nil {
		return err
	}
	defer pool.Close()
	return katosync.Migrate(ctx, pool, log)
}

func syncConfig() (katosync.Config, error) {
	offline, err := strconv.ParseBool(env("OFFLINE", "true"))
	if err != nil {
		return katosync.Config{}, fmt.Errorf("OFFLINE: %w", err)
	}
	interval, err := time.ParseDuration(env("KATO_REFRESH_INTERVAL", "168h"))
	if err != nil || interval < time.Hour {
		return katosync.Config{}, fmt.Errorf("KATO_REFRESH_INTERVAL must be a duration of at least 1h, got %q", os.Getenv("KATO_REFRESH_INTERVAL"))
	}
	return katosync.Config{Offline: offline, Interval: interval, PageURL: env("KATO_PAGE_URL", katosync.DefaultPage)}, nil
}

func healthcheck() error {
	addr := env("HTTP_ADDR", ":8080")
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("bad HTTP_ADDR %q: %w", addr, err)
	}
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/readyz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("readyz returned %d", resp.StatusCode)
	}
	return nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
