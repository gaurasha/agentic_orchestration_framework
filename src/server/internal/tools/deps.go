package tools

import "context"

type Store interface {
	Get(ctx context.Context, tenant, name string) (Tool, error) // ErrNotFound if absent
	Put(ctx context.Context, t Tool) error
	List(ctx context.Context, tenant string) ([]Tool, error) // sorted by name
}

type Deps struct {
	Store Store
}
