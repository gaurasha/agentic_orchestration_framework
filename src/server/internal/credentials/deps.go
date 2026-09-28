package credentials

import (
	"context"
	"time"
)

type Clock interface{ Now() time.Time }

// Secrets stores real credentials: Vault in production, SQLite locally.
type Secrets interface {
	Get(ctx context.Context, tenant, ref string) (Secret, error) // ErrNotFound if absent
	Put(ctx context.Context, tenant, ref string, s Secret) error
}

// Keys signs tokens. Each token names its key, so keys can rotate.
type Keys interface {
	Current(ctx context.Context) (id string, key Secret, err error)
	Get(ctx context.Context, id string) (Secret, error)
}

type Deps struct {
	Secrets Secrets
	Keys    Keys
	Clock   Clock
}
