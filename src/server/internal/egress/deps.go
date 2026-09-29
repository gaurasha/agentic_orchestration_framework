package egress

import (
	"context"
	"log/slog"
	"net/http"
)

// Secret is a credential value; an implementation must never print it.
type Secret interface{ Reveal() string }

// Placeholders exchanges a live placeholder for the real credential it
// stands for, if host is one it allows.
type Placeholders interface {
	Resolve(ctx context.Context, value, host string) (Secret, error) // ErrUnauthenticated or ErrDenied
}

// Upstream sends the forwarded request. It must not follow redirects, so
// the credential reaches only the host the placeholder allows.
type Upstream interface {
	Do(ctx context.Context, r *http.Request) (*http.Response, error)
}

type Deps struct {
	Placeholders Placeholders
	Upstream     Upstream
	Log          *slog.Logger // never given a secret
}
