// Command server runs everything in one process: the HTTP API the UI calls,
// the tool gateway and the in-memory agent runner.
//
// It is the only place that reads the environment and the only place that
// imports more than one domain; wire.go adapts each domain's API to the
// interfaces the others need. The domains have interfaces only so far, so
// the server answers /healthz and nothing else.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type config struct {
	Addr   string // where the HTTP API listens
	DB     string // the SQLite file
	LLMKey string // a real provider's key; empty selects the scripted mock
}

func main() {
	cfg := config{
		Addr:   env("ADDR", ":8080"),
		DB:     env("DB", "agentic_orchestration_framework.db"),
		LLMKey: os.Getenv("LLM_API_KEY"),
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cfg); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	srv := &http.Server{Addr: cfg.Addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	slog.Info("listening", "addr", cfg.Addr, "db", cfg.DB, "llm", provider(cfg))

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// provider names the LLM provider the config selects, never the key.
func provider(cfg config) string {
	if cfg.LLMKey == "" {
		return "mock"
	}
	return "real"
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
