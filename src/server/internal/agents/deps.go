package agents

import (
	"context"
	"time"
)

type Clock interface{ Now() time.Time }

type IDGen interface{ NewID() string }

// Store keeps versions and never changes one.
type Store interface {
	Get(ctx context.Context, tenant, id string, version int) (Agent, error) // ErrNotFound if absent
	Latest(ctx context.Context, tenant, id string) (Agent, error)           // ErrNotFound if absent
	List(ctx context.Context, tenant string) ([]Agent, error)
	// Put stores a new version; ErrConflict if that version exists.
	Put(ctx context.Context, a Agent) error
}

// Registry says which tools a tenant has.
type Registry interface {
	Has(ctx context.Context, tenant, tool string) (bool, error)
}

type Deps struct {
	Store    Store
	Registry Registry
	Clock    Clock
	IDs      IDGen
}
