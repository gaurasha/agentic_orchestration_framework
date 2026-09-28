// Package credentials issues and checks the short-lived tokens callers hold,
// and keeps the real credentials only the gateway sees. A real credential
// is looked up only at the moment of an external request.
package credentials

import (
	"context"
	"errors"
	"time"
)

var (
	ErrUnauthenticated = errors.New("unauthenticated")      // forged or expired
	ErrNotFound        = errors.New("credential not found") // nothing under the reference
)

type API interface {
	Issue(ctx context.Context, c Claims, ttl time.Duration) (string, error)
	Verify(ctx context.Context, token string) (Claims, error)
	// Resolve returns a real credential. Only the gateway calls it.
	Resolve(ctx context.Context, tenant, ref string) (Secret, error)
	Store(ctx context.Context, tenant, ref string, s Secret) error
}
