// Package sandbox runs commands in one sandbox per execution. For now a
// sandbox is mocked or a local process, with no isolation; a real runtime
// replaces it behind Runtime.
package sandbox

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("sandbox not found")

type API interface {
	// Exec starts the execution's sandbox on first use.
	Exec(ctx context.Context, tenant, execution string, cmd Command) (Output, error)
	// Release removes the sandbox once the execution ends.
	Release(ctx context.Context, execution string) error
}
