// Package memory holds a fixed set of placeholders for tests.
package memory

import (
	"context"
	"fmt"
	"slices"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/egress"
)

// Placeholder is one live placeholder and what it stands for.
type Placeholder struct {
	Real  string
	Hosts []string
}

type Placeholders map[string]Placeholder

type secret string

func (s secret) Reveal() string { return string(s) }
func (secret) String() string   { return "[secret]" }

func (p Placeholders) Resolve(_ context.Context, value, host string) (egress.Secret, error) {
	ph, ok := p[value]
	if !ok {
		return nil, fmt.Errorf("%w: unknown placeholder", egress.ErrUnauthenticated)
	}
	if !slices.Contains(ph.Hosts, host) {
		return nil, fmt.Errorf("%w: placeholder is not for host %s", egress.ErrDenied, host)
	}
	return secret(ph.Real), nil
}
