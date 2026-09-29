package tools

import (
	"context"
	"fmt"
)

type service struct{ d Deps }

func (s *service) Put(ctx context.Context, t Tool) (Tool, error) {
	t = normalize(t)
	if err := validate(t); err != nil {
		return Tool{}, err
	}
	if err := s.d.Store.Put(ctx, t); err != nil {
		return Tool{}, fmt.Errorf("put tool %s: %w", t.Name, err)
	}
	return t, nil
}

func (s *service) Get(ctx context.Context, tenant, name string) (Tool, error) {
	return s.d.Store.Get(ctx, tenant, name)
}

func (s *service) List(ctx context.Context, tenant string) ([]Tool, error) {
	return s.d.Store.List(ctx, tenant)
}
