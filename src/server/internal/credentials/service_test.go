package credentials_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/credentials"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/credentials/deps/memory"
)

type clock struct{ t time.Time }

func (c *clock) Now() time.Time { return c.t }

type random struct{ n int }

func (r *random) Token() string { r.n++; return fmt.Sprintf("%032d", r.n) }

// 5.3: a placeholder stops working when retired or expired, and only ever
// works for its own hosts.
func TestPlaceholderLifetime(t *testing.T) {
	ctx := context.Background()
	clk := &clock{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	api := credentials.New(credentials.Deps{
		Secrets: memory.NewSecrets(), Placeholders: memory.NewPlaceholders(),
		Key: credentials.NewSecret("k"), Clock: clk, Random: &random{},
	})
	if err := api.Store(ctx, "acme", "gh", credentials.NewSecret("real-token")); err != nil {
		t.Fatal(err)
	}
	p := credentials.Placeholder{Tenant: "acme", Execution: "e1", Ref: "gh", Hosts: []string{"api.github.com"}, Expires: clk.t.Add(time.Minute)}

	if _, err := api.IssuePlaceholder(ctx, credentials.Placeholder{Tenant: "acme", Execution: "e1", Ref: "missing", Hosts: p.Hosts, Expires: p.Expires}); !errors.Is(err, credentials.ErrNotFound) {
		t.Errorf("unknown ref: err = %v, want ErrNotFound", err)
	}
	value, err := api.IssuePlaceholder(ctx, p)
	if err != nil || !strings.HasPrefix(value, credentials.PlaceholderPrefix) {
		t.Fatalf("IssuePlaceholder = %q, %v", value, err)
	}
	if sec, err := api.ResolvePlaceholder(ctx, value, "api.github.com"); err != nil || sec.Reveal() != "real-token" {
		t.Errorf("live: got %v, %v", sec, err)
	}
	if _, err := api.ResolvePlaceholder(ctx, value, "evil.example"); !errors.Is(err, credentials.ErrDenied) {
		t.Errorf("other host: err = %v, want ErrDenied", err)
	}
	if _, err := api.ResolvePlaceholder(ctx, "fake_forged", "api.github.com"); !errors.Is(err, credentials.ErrUnauthenticated) {
		t.Errorf("forged: err = %v, want ErrUnauthenticated", err)
	}

	clk.t = clk.t.Add(2 * time.Minute)
	if _, err := api.ResolvePlaceholder(ctx, value, "api.github.com"); !errors.Is(err, credentials.ErrUnauthenticated) {
		t.Errorf("expired: err = %v, want ErrUnauthenticated", err)
	}
	clk.t = clk.t.Add(-2 * time.Minute)
	if err := api.RetirePlaceholder(ctx, value); err != nil {
		t.Fatal(err)
	}
	if _, err := api.ResolvePlaceholder(ctx, value, "api.github.com"); !errors.Is(err, credentials.ErrUnauthenticated) {
		t.Errorf("retired: err = %v, want ErrUnauthenticated", err)
	}
}
