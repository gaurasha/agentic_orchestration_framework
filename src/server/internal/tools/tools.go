// Package tools is the tool registry: each tenant's tools, kept as data, so
// a tenant adds a tool without a release or a restart.
package tools

import (
	"context"
	"errors"
)

var (
	ErrNotFound = errors.New("tool not found")
	ErrInvalid  = errors.New("invalid tool") // e.g. an HTTP tool with no hosts
)

type API interface {
	Put(ctx context.Context, t Tool) error
	Get(ctx context.Context, tenant, name string) (Tool, error)
	List(ctx context.Context, tenant string) ([]Tool, error)
}
