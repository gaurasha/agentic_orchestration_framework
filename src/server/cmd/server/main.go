// Command server runs everything in one process: the HTTP API the UI calls,
// the tool gateway, the agent loop, a fake external API for the demo, and
// on a second port the egress endpoint sandboxed commands send their
// requests through. Everything is in memory; a restart wipes it.
//
// It is the only place that reads the environment and the only place that
// imports more than one domain; wire.go adapts each domain's API to the
// interfaces the others need.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

type config struct {
	Addr         string // where the HTTP API listens
	EgressAddr   string // where the egress endpoint listens
	EgressURL    string // how a sandboxed command reaches it; Docker needs host.docker.internal
	CAFile       string // where the platform CA's certificate is written for the sandbox to trust
	Sandbox      string // "local" (processes on this machine) or "docker"
	SandboxImage string // for Docker; see `make sandbox-image`
	Seed         string // the tenant to load the demo data for at start; "" for none
	GitHubToken  string // GH_CLI_TOKEN: stored as the seed tenant's github_token, for the gh demo; never logged
}

func main() {
	sandbox := env("SANDBOX", "local")
	egressAddr := env("EGRESS_ADDR", ":8081")
	// The default EGRESS_URL follows the egress listener's port, so moving
	// the listener never sends the sandbox to another server's egress.
	_, egressPort, err := net.SplitHostPort(egressAddr)
	if err != nil {
		egressPort = "8081"
	}
	egressURL := "http://localhost:" + egressPort
	if sandbox == "docker" {
		egressURL = "http://host.docker.internal:" + egressPort
	}
	// Under the home directory, which Docker on a Mac (Colima, Docker
	// Desktop) shares with its VM, so the file can be mounted into a container.
	caFile := filepath.Join(os.TempDir(), "aof-egress-ca.pem")
	if dir, err := os.UserCacheDir(); err == nil {
		caFile = filepath.Join(dir, "aof", "egress-ca.pem")
	}
	cfg := config{
		Addr:         env("ADDR", ":8080"),
		EgressAddr:   egressAddr,
		EgressURL:    env("EGRESS_URL", egressURL),
		CAFile:       env("EGRESS_CA_FILE", caFile),
		Sandbox:      sandbox,
		SandboxImage: env("SANDBOX_IMAGE", "aof-sandbox"),
		Seed:         env("SEED", "acme"),
		GitHubToken:  os.Getenv("GH_CLI_TOKEN"),
	}
	if cfg.Seed == "none" {
		cfg.Seed = ""
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cfg, slog.Default()); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config, log *slog.Logger) error {
	key := make([]byte, 32) // a fresh signing key per process: tokens die with it
	if _, err := rand.Read(key); err != nil {
		return err
	}
	app, err := newApp(appConfig{
		SigningKey: hex.EncodeToString(key), EgressURL: cfg.EgressURL, CAFile: cfg.CAFile,
		Sandbox: cfg.Sandbox, SandboxImage: cfg.SandboxImage, AllowHTTPEgress: true,
	}, log, realClock{})
	if err != nil {
		return err
	}
	api := &http.Server{Addr: cfg.Addr, Handler: app.handler(), ReadHeaderTimeout: 5 * time.Second}
	egress := &http.Server{Addr: cfg.EgressAddr, Handler: app.egressHandler(), ReadHeaderTimeout: 5 * time.Second}

	// Listen before serving, so the seed's runs can reach /demo/echo and
	// egress as soon as the servers start.
	apiLn, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	egressLn, err := net.Listen("tcp", cfg.EgressAddr)
	if err != nil {
		_ = apiLn.Close()
		return err
	}
	errc := make(chan error, 2)
	go func() { errc <- api.Serve(apiLn) }()
	go func() { errc <- egress.Serve(egressLn) }()
	log.Info("listening", "api", cfg.Addr, "egress", cfg.EgressAddr, "sandbox", cfg.Sandbox, "ca", cfg.CAFile)

	if cfg.Seed != "" {
		apiHost := net.JoinHostPort("localhost", strconv.Itoa(apiLn.Addr().(*net.TCPAddr).Port))
		// A seed that fails leaves a working, emptier server, so it is logged, not fatal.
		docker := cfg.Sandbox == "docker"
		if err := app.seed(ctx, seedOptions{Tenant: cfg.Seed, APIHost: apiHost, RunCommands: !docker, GitHubToken: cfg.GitHubToken, RunGitHub: docker}); err != nil {
			log.Error("seed failed", "tenant", cfg.Seed, "err", err)
		} else {
			log.Info("seeded demo data", "tenant", cfg.Seed)
		}
	}

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, srv := range []*http.Server{api, egress} {
		if err := srv.Shutdown(shutdown); err != nil {
			return err
		}
		if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
