package sandbox

import (
	"context"
	"time"
)

// Clock tells the time, so idle checks are testable.
type Clock interface{ Now() time.Time }

// Runtime starts and stops sandboxes: Kubernetes pods in production,
// Docker containers locally.
type Runtime interface {
	Start(ctx context.Context, s Spec) (pod string, err error)
	Exec(ctx context.Context, pod string, cmd Command) (Output, error)
	Stop(ctx context.Context, pod string) error
}

// Drives hold each execution's working directory: a persistent volume in
// production, a named Docker volume locally.
type Drives interface {
	Create(ctx context.Context, execution string) (drive string, err error)
	Delete(ctx context.Context, drive string) error
}

// Backups copy a drive to object storage and back, e.g. with restic.
type Backups interface {
	Save(ctx context.Context, execution, drive string) (snapshot string, err error)
	Restore(ctx context.Context, snapshot, drive string) error
	// Forget deletes every backup of the execution.
	Forget(ctx context.Context, execution string) error
}

// Store remembers each execution's sandbox.
type Store interface {
	// Get returns ErrNotFound if the execution has no sandbox.
	Get(ctx context.Context, execution string) (Sandbox, error)
	Put(ctx context.Context, s Sandbox) error
	// IdleSince lists running sandboxes with no activity since t.
	IdleSince(ctx context.Context, t time.Time) ([]Sandbox, error)
}

// Deps is everything the sandbox manager needs.
type Deps struct {
	Runtime Runtime
	Drives  Drives
	Backups Backups
	Store   Store
	Clock   Clock
}
