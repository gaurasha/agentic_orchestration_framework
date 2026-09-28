package gateway

import (
	"context"
	"net/http"
)

type Tokens interface {
	Verify(ctx context.Context, token string) (Caller, error) // ErrUnauthenticated if bad
}

type Catalog interface {
	Tool(ctx context.Context, tenant, name string) (Tool, error)
}

type Grants interface {
	For(ctx context.Context, c Caller) (Grant, error)
}

type Budgets interface {
	// Spend counts one call, or returns ErrExhausted and counts nothing.
	Spend(ctx context.Context, execution string, limit Budget) error
}

// Journal makes calls idempotent.
type Journal interface {
	// Begin claims the key. A recorded result comes back with done set; a
	// first call still running is ErrConflict.
	Begin(ctx context.Context, key string) (r Result, done bool, err error)
	Finish(ctx context.Context, key string, r Result) error
}

// Secret is a credential value; an implementation must never print it.
type Secret interface{ Reveal() string }

type Credentials interface {
	Resolve(ctx context.Context, tenant, ref string) (Secret, error)
}

type Sandbox interface {
	Exec(ctx context.Context, tenant, execution string, cmd Command) (Output, error)
}

// Upstream sends a checked request, credential added, to an external service.
type Upstream interface {
	Do(ctx context.Context, r *http.Request) (*http.Response, error)
}

type Deps struct {
	Tokens      Tokens
	Catalog     Catalog
	Grants      Grants
	Budgets     Budgets
	Journal     Journal
	Credentials Credentials
	Sandbox     Sandbox
	Upstream    Upstream
}
