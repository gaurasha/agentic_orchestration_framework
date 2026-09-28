package audit

import (
	"context"
	"time"
)

// Clock tells the time.
type Clock interface{ Now() time.Time }

// Store keeps records in order and never changes one.
type Store interface {
	// Last returns the tenant's newest record, or ErrNotFound if the
	// chain is empty.
	Last(ctx context.Context, tenant string) (Record, error)
	// Append stores r. It returns ErrConflict unless r.Seq is one
	// past the newest record, so two writers can never fork the chain.
	Append(ctx context.Context, r Record) error
	// Scan calls fn on each of the tenant's records in order, and stops at
	// the first error fn returns.
	Scan(ctx context.Context, tenant string, fn func(Record) error) error
}

// Deps is everything the audit log needs.
type Deps struct {
	Store Store
	Clock Clock
}
