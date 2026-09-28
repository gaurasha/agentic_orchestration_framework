// Package credentials issues and checks the short-lived tokens callers
// hold, and keeps the real credentials only the gateway ever sees.
//
// A token proves who is calling: tenant, execution and pinned agent
// version. A real credential is looked up by reference only at the moment
// of an external request.
//
// Needs: the interfaces in deps.go.
// Portability: interfaces only so far; no logic, service or adapters yet.
package credentials

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Errors, matched with errors.Is.
var (
	ErrUnauthenticated = errors.New("unauthenticated")      // token forged or expired
	ErrNotFound        = errors.New("credential not found") // nothing stored under the reference
)

// API is what the credentials domain offers.
type API interface {
	// Issue mints a short-lived token that proves the claims.
	Issue(ctx context.Context, c Claims, ttl time.Duration) (string, error)
	// Verify checks a token and returns its claims. A forged or expired
	// token is ErrUnauthenticated.
	Verify(ctx context.Context, token string) (Claims, error)
	// Resolve returns the real credential stored under a reference. Only
	// the gateway calls it, just before an external request.
	Resolve(ctx context.Context, tenant, ref string) (Secret, error)
	// Store saves a real credential under a reference.
	Store(ctx context.Context, tenant, ref string, s Secret) error
}

// Claims are what a token proves about whoever holds it.
type Claims struct {
	ID           string // unique per token, for the audit trail
	Tenant       string
	Execution    string
	AgentVersion string
	Expires      time.Time
}

// Secret holds a credential value. It prints as "[secret]" under every fmt
// verb, and keeps the value behind a pointer, so even a struct holding it in
// an unexported field prints an address, never the value. Reveal returns the
// value; call it only where the value is used.
type Secret struct{ p *string }

// NewSecret wraps a credential value.
func NewSecret(v string) Secret { return Secret{p: &v} }

// Reveal returns the credential value, or "" for the zero Secret.
func (s Secret) Reveal() string {
	if s.p == nil {
		return ""
	}
	return *s.p
}

// String returns a placeholder, never the value.
func (s Secret) String() string { return "[secret]" }

// Format prints the placeholder for every verb, including %#v and %d.
func (s Secret) Format(f fmt.State, _ rune) { _, _ = fmt.Fprint(f, "[secret]") }
