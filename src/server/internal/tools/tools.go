// Package tools is the tool registry: each tenant's tools, HTTP requests or
// sandboxed commands, kept as data, so a tenant adds a tool without a
// release or a restart.
package tools

import (
	"context"
	"errors"
)

var (
	ErrNotFound = errors.New("tool not found")
	ErrInvalid  = errors.New("invalid tool") // e.g. a placeholder in the host
)

type API interface {
	// Put adds the tool or replaces the one with its name.
	Put(ctx context.Context, t Tool) (Tool, error)
	Get(ctx context.Context, tenant, name string) (Tool, error)
	List(ctx context.Context, tenant string) ([]Tool, error)
}

func New(d Deps) API { return &service{d: d} }
