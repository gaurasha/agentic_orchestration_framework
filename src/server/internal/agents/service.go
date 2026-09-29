package agents

import (
	"context"
	"fmt"
)

type service struct{ d Deps }

func (s *service) Create(ctx context.Context, tenant string, def Definition) (Agent, error) {
	def, err := s.check(ctx, tenant, def)
	if err != nil {
		return Agent{}, err
	}
	a := Agent{Definition: def, ID: s.d.IDs.NewID(), Version: 1, Tenant: tenant, Hash: hash(def), CreatedAt: s.d.Clock.Now()}
	if err := s.d.Store.Put(ctx, a); err != nil {
		return Agent{}, fmt.Errorf("create agent: %w", err)
	}
	return a, nil
}

func (s *service) Update(ctx context.Context, tenant, id string, def Definition) (Agent, error) {
	latest, err := s.d.Store.Latest(ctx, tenant, id)
	if err != nil {
		return Agent{}, err
	}
	def, err = s.check(ctx, tenant, def)
	if err != nil {
		return Agent{}, err
	}
	h := hash(def)
	if h == latest.Hash {
		return latest, nil
	}
	a := Agent{Definition: def, ID: id, Version: latest.Version + 1, Tenant: tenant, Hash: h, CreatedAt: s.d.Clock.Now()}
	if err := s.d.Store.Put(ctx, a); err != nil {
		return Agent{}, fmt.Errorf("update agent %s: %w", id, err)
	}
	return a, nil
}

func (s *service) Get(ctx context.Context, tenant, id string, version int) (Agent, error) {
	return s.d.Store.Get(ctx, tenant, id, version)
}

func (s *service) Latest(ctx context.Context, tenant, id string) (Agent, error) {
	return s.d.Store.Latest(ctx, tenant, id)
}

func (s *service) List(ctx context.Context, tenant string) ([]Agent, error) {
	return s.d.Store.List(ctx, tenant)
}

func (s *service) Versions(ctx context.Context, tenant, id string) ([]Agent, error) {
	return s.d.Store.Versions(ctx, tenant, id)
}

// check normalizes and validates, then refuses a tool the tenant lacks.
func (s *service) check(ctx context.Context, tenant string, def Definition) (Definition, error) {
	def = normalize(def)
	if err := validate(def); err != nil {
		return Definition{}, err
	}
	for _, t := range def.Tools {
		ok, err := s.d.Registry.Has(ctx, tenant, t.Name)
		if err != nil {
			return Definition{}, fmt.Errorf("check tool %s: %w", t.Name, err)
		}
		if !ok {
			return Definition{}, fmt.Errorf("%w: tool %s is not in the registry", ErrInvalid, t.Name)
		}
	}
	return def, nil
}
