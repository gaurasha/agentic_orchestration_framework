package timeline

import (
	"context"
	"time"
)

type Clock interface{ Now() time.Time }

type Store interface {
	// Append stores e as the execution's next entry.
	Append(ctx context.Context, e Event, at time.Time) (Entry, error)
	List(ctx context.Context, tenant, execution string) ([]Entry, error)
}

type Deps struct {
	Store Store
	Clock Clock
}
