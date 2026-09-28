package credentials

import (
	"context"
	"time"
)

// Clock tells the time, so expiry is testable.
type Clock interface{ Now() time.Time }

// IDGen makes token IDs.
type IDGen interface{ NewID() string }

// Secrets stores real credentials: a secrets manager such as Vault in
// production, SQLite or memory locally.
type Secrets interface {
	// Get returns ErrNotFound if nothing is stored under the reference.
	Get(ctx context.Context, tenant, ref string) (Secret, error)
	Put(ctx context.Context, tenant, ref string, s Secret) error
}

// Keys holds the keys that sign tokens. Each token names its key, so a key
// can rotate while tokens signed with the old one are still valid.
type Keys interface {
	Current(ctx context.Context) (id string, key Secret, err error)
	Get(ctx context.Context, id string) (Secret, error)
}

// Deps is everything the credentials domain needs.
type Deps struct {
	Secrets Secrets
	Keys    Keys
	Clock   Clock
	IDs     IDGen
}
