package gateway

import (
	"context"
	"net/http"
	"time"
)

// Clock tells the time, so logic and tests never read the real clock.
type Clock interface{ Now() time.Time }

// IDGen makes unique IDs, e.g. to tie a call's audit events together.
type IDGen interface{ NewID() string }

// Tokens verifies the short-lived token a caller presents.
// A forged or expired token is ErrUnauthenticated.
type Tokens interface {
	Verify(ctx context.Context, token string) (Caller, error)
}

// Catalog holds the tool definitions.
type Catalog interface {
	Tool(ctx context.Context, tenant, name string) (Tool, error)
}

// Grants says what an execution may do.
type Grants interface {
	For(ctx context.Context, execution string) (Grant, error)
}

// Budgets counts what each execution has spent.
type Budgets interface {
	// Spend counts one call against the execution, or returns
	// ErrExhausted, counting nothing, if the budget is used up.
	Spend(ctx context.Context, execution string, limit Budget) error
}

// Journal makes calls idempotent: the first call with a key runs, and a
// retry with the same key gets the recorded result.
type Journal interface {
	// Begin claims the key. If a result is already recorded, it returns it
	// with done set. If the first call is still running, it returns
	// ErrConflict.
	Begin(ctx context.Context, key string) (r Result, done bool, err error)
	// Finish records the result for the key.
	Finish(ctx context.Context, key string, r Result) error
}

// Audit records every decision and outcome.
type Audit interface {
	Record(ctx context.Context, e Event) error
}

// Secret is a credential value. The gateway reveals it only into the
// external request; an implementation must never print the value.
type Secret interface{ Reveal() string }

// Credentials swaps a credential reference for the real secret, at the
// moment of the external request.
type Credentials interface {
	Resolve(ctx context.Context, tenant, ref string) (Secret, error)
}

// Sandbox runs commands in an execution's sandbox.
type Sandbox interface {
	Exec(ctx context.Context, tenant, execution string, cmd Command) (Output, error)
}

// Upstream sends a request to an external service, after the gateway has
// checked it and added the credential.
type Upstream interface {
	Do(ctx context.Context, r *http.Request) (*http.Response, error)
}

// Deps is everything the gateway needs, handed in by whoever builds it.
type Deps struct {
	Tokens      Tokens
	Catalog     Catalog
	Grants      Grants
	Budgets     Budgets
	Journal     Journal
	Audit       Audit
	Credentials Credentials
	Sandbox     Sandbox
	Upstream    Upstream
	Clock       Clock
	IDs         IDGen
}
