package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type Catalog interface {
	Tool(ctx context.Context, tenant, name string) (Tool, error) // ErrDenied if absent
}

type Grants interface {
	For(ctx context.Context, c Caller) (Grant, error)
}

type Budgets interface {
	// Spend counts one call, or returns ErrExhausted and counts nothing.
	Spend(ctx context.Context, execution string, limit Budget) error
}

// Journal makes calls idempotent, per tenant.
type Journal interface {
	// Begin claims the key. A saved result comes back with done set; a
	// first call still running is ErrConflict.
	Begin(ctx context.Context, tenant, key string) (r Result, done bool, err error)
	Finish(ctx context.Context, tenant, key string, r Result) error
	// Abort releases a claim whose call did not run, so a retry can.
	Abort(ctx context.Context, tenant, key string) error
}

// Approvals keeps a human's approvals on file, by call key, per tenant.
type Approvals interface {
	Has(ctx context.Context, tenant, key string) (bool, error)
	Put(ctx context.Context, tenant, key string) error
}

// Secret is a credential value; an implementation must never print it.
type Secret interface{ Reveal() string }

type Credentials interface {
	Resolve(ctx context.Context, tenant, ref string) (Secret, error)
}

// Placeholders issues the value a sandbox holds in place of a credential,
// and retires it when the call ends.
type Placeholders interface {
	Issue(ctx context.Context, p PlaceholderRequest) (string, error)
	Retire(ctx context.Context, value string) error
}

type PlaceholderRequest struct {
	Tenant    string
	Execution string
	Ref       string
	Hosts     []string
	TTL       time.Duration
}

// Sandbox runs a command in the execution's sandbox.
type Sandbox interface {
	Exec(ctx context.Context, tenant, execution string, cmd Command) (Output, error)
}

// Upstream sends a checked request, credential added, to an external
// service. It must not follow redirects, so the credential reaches only the
// host the tool names.
type Upstream interface {
	Do(ctx context.Context, r *http.Request) (*http.Response, error)
}

type Deps struct {
	Catalog      Catalog
	Grants       Grants
	Budgets      Budgets
	Journal      Journal
	Approvals    Approvals
	Credentials  Credentials
	Placeholders Placeholders
	Sandbox      Sandbox
	Upstream     Upstream
	Log          *slog.Logger // never given a secret
}
