package sandbox

import "context"

// Runtime runs sandboxes: for now a mock or local processes.
type Runtime interface {
	Start(ctx context.Context, tenant, execution string) (id string, err error)
	Exec(ctx context.Context, id string, cmd Command) (Output, error)
	Stop(ctx context.Context, id string) error
}

// Store remembers each execution's sandbox.
type Store interface {
	Get(ctx context.Context, execution string) (id string, err error) // ErrNotFound if absent
	Put(ctx context.Context, execution, id string) error
	Delete(ctx context.Context, execution string) error
}

type Deps struct {
	Runtime Runtime
	Store   Store
}
