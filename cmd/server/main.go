// Command server runs the Jana Address HTTP API.
//
// Subcommands:
//
//	serve        start the HTTP server (default); migrates the database and loads KATO in the background
//	migrate      apply the database migrations and seed the KATO snapshot, then exit
//	healthcheck  probe /readyz of a running server; used by the Docker HEALTHCHECK (distroless has no shell)
//	synth        write the generated test set to stdout: synth -n 10000 -seed 1 [-format jury|gold]
//	version      print the build version
//
// Environment: HTTP_ADDR, DATABASE_URL (PGUSER, PGPASSWORD, PGDATABASE fill what it leaves out),
// OFFLINE (default true: never call stat.gov.kz),
// KATO_REFRESH_INTERVAL (default 168h), KATO_PAGE_URL.
package main

import (
	"context"
	"errors"
	"flag"
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
	"github.com/jana-address/jana-address-api/internal/parser/dict"
	"github.com/jana-address/jana-address-api/internal/synth"
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
	case "synth":
		err = synthSet(os.Args[2:])
	case "version":
		fmt.Println(version)
	default:
		err = fmt.Errorf("unknown command %q (want serve, migrate, healthcheck, synth or version)", cmd)
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
	start := time.Now()
	osm := dict.OSM() // the street directory is read before the server listens, not on the first request
	log.Info("osm directory loaded", "cities", len(osm.Streets), "ms", time.Since(start).Milliseconds())

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
		// a batch in flight gets time to finish; docker-compose waits 40 s (stop_grace_period) before SIGKILL
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

// synthSet writes the generated set (the one the tests score) to stdout, so the 10k file for the UI
// can be made from the image, which has no Go toolchain to run cmd/synth.
func synthSet(args []string) error {
	fs := flag.NewFlagSet("synth", flag.ContinueOnError)
	n := fs.Int("n", 10000, "number of records")
	seed := fs.Uint64("seed", 1, "random seed")
	format := fs.String("format", "jury", "jury: JSON array of id and raw_address; gold: JSONL with labels")
	if err := fs.Parse(args); err != nil {
		return err
	}
	recs := synth.Generate(*n, *seed)
	switch *format {
	case "jury":
		return synth.WriteJury(os.Stdout, recs)
	case "gold":
		return synth.WriteGold(os.Stdout, recs)
	}
	return fmt.Errorf("unknown format %q (want jury or gold)", *format)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
