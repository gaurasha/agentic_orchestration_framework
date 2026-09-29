// Package sandbox runs commands in one sandbox per execution, started on
// first use and released when the execution ends. A command gets only the
// environment it is given: never a real credential, only a placeholder.
// The runtime is a local process, with no isolation, or a Docker container.
package sandbox

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("sandbox not found")

type API interface {
	// Exec starts the execution's sandbox on first use and runs cmd in it.
	Exec(ctx context.Context, tenant, execution string, cmd Command) (Output, error)
	// Release removes the sandbox once the execution ends.
	Release(ctx context.Context, execution string) error
}

func New(d Deps) API { return &service{d: d} }
