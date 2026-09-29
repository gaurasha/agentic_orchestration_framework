// Package agents keeps agent definitions. An agent has a stable ID; each
// edit makes a new numbered version, and a version never changes, so an
// execution keeps the version it started on. Saving content identical to
// the latest version makes no new version.
package agents

import (
	"context"
	"errors"
)

var (
	ErrNotFound = errors.New("agent not found")
	ErrInvalid  = errors.New("invalid definition") // e.g. it lists a tool the tenant lacks
	ErrConflict = errors.New("version conflict")   // another edit saved that version first
)

type API interface {
	// Create makes a new agent at version 1.
	Create(ctx context.Context, tenant string, def Definition) (Agent, error)
	// Update makes the agent's next version, or returns the latest if def
	// is unchanged.
	Update(ctx context.Context, tenant, id string, def Definition) (Agent, error)
	Get(ctx context.Context, tenant, id string, version int) (Agent, error)
	Latest(ctx context.Context, tenant, id string) (Agent, error)
	// List returns each agent at its latest version.
	List(ctx context.Context, tenant string) ([]Agent, error)
	// Versions returns every version of one agent, oldest first.
	Versions(ctx context.Context, tenant, id string) ([]Agent, error)
}

func New(d Deps) API { return &service{d: d} }
