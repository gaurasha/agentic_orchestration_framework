package credentials

import (
	"context"
	"time"
)

type Clock interface{ Now() time.Time }

// Random makes unguessable tokens.
type Random interface{ Token() string }

// Secrets stores real credentials: in memory for now, a vault in production.
type Secrets interface {
	Get(ctx context.Context, tenant, ref string) (Secret, error) // ErrNotFound if absent
	Put(ctx context.Context, tenant, ref string, s Secret) error
	List(ctx context.Context, tenant string) ([]string, error) // sorted refs
}

// Placeholders keeps live placeholders by value.
type Placeholders interface {
	Get(ctx context.Context, value string) (Placeholder, error) // ErrUnauthenticated if absent
	Put(ctx context.Context, value string, p Placeholder) error
	Delete(ctx context.Context, value string) error
}

type Deps struct {
	Secrets      Secrets
	Placeholders Placeholders
	Key          Secret // signs tokens
	Clock        Clock
	Random       Random
}
