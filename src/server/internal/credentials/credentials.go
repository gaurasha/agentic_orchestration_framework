// Package credentials keeps the real credentials only the gateway sees,
// signs the short-lived token the UI holds for its tenant, and issues
// placeholders: values a sandbox may hold in place of a credential, which
// the egress endpoint exchanges for the real one on a request to an allowed
// host, until the tool call ends. A stored value can be written and
// resolved, never listed or returned.
package credentials

import (
	"context"
	"errors"
	"time"
)

var (
	ErrUnauthenticated = errors.New("unauthenticated")      // forged, expired or retired
	ErrDenied          = errors.New("denied")               // a placeholder used for a host it does not allow
	ErrNotFound        = errors.New("credential not found") // nothing under the reference
	ErrInvalid         = errors.New("invalid credential")   // empty reference or value
)

type API interface {
	// Issue signs a token that names the tenant until ttl passes.
	Issue(ctx context.Context, tenant string, ttl time.Duration) (string, error)
	Verify(ctx context.Context, token string) (Claims, error)
	// Resolve returns a real credential. Only the gateway calls it.
	Resolve(ctx context.Context, tenant, ref string) (Secret, error)
	Store(ctx context.Context, tenant, ref string, s Secret) error
	// List names the tenant's credentials, never their values.
	List(ctx context.Context, tenant string) ([]string, error)

	// IssuePlaceholder returns a value that stands for p.Ref until p.Expires
	// or RetirePlaceholder, on requests to p.Hosts only.
	IssuePlaceholder(ctx context.Context, p Placeholder) (string, error)
	// ResolvePlaceholder returns the real credential a live placeholder
	// stands for: ErrUnauthenticated if it is unknown, expired or retired,
	// ErrDenied if host is not one it allows.
	ResolvePlaceholder(ctx context.Context, value, host string) (Secret, error)
	RetirePlaceholder(ctx context.Context, value string) error
}

func New(d Deps) API { return &service{d: d} }
