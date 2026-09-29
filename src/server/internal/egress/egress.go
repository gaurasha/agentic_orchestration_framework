// Package egress is the way out for a sandboxed command. The command sends
// its request here with a placeholder where the credential goes; egress
// checks that the placeholder is live and allows the host, exchanges it for
// the real credential, forwards the request to the host, and hides the real
// credential again in whatever comes back. The real credential never enters
// the sandbox, and a placeholder never works for another host.
package egress

import (
	"context"
	"errors"
	"log/slog"
)

var (
	ErrUnauthenticated = errors.New("unauthenticated") // no live placeholder in the request
	ErrDenied          = errors.New("denied")          // the placeholder is not for this host, or the scheme is not allowed
	ErrTooLarge        = errors.New("too large")       // a body over Config.MaxBody, either way; never truncated silently
)

type API interface {
	Forward(ctx context.Context, r Request) (Response, error)
}

func New(d Deps, cfg Config) API {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if cfg.MaxBody <= 0 {
		cfg.MaxBody = 1 << 20
	}
	return &service{d: d, cfg: cfg}
}
