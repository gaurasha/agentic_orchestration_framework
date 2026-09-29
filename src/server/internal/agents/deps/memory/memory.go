// Package memory is the in-memory agent Store, plus a Registry that holds
// a fixed set of tool names for tests; a restart wipes it.
package memory

import (
	"context"
	"sync"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/agents"
)

type Store struct {
	mu    sync.Mutex
	m     map[string]map[string][]agents.Agent // tenant -> id -> versions, oldest first
	order map[string][]string                  // tenant -> ids by creation
}

func NewStore() *Store {
	return &Store{m: map[string]map[string][]agents.Agent{}, order: map[string][]string{}}
}

func (s *Store) Get(_ context.Context, tenant, id string, version int) (agents.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vs := s.m[tenant][id]
	if version < 1 || version > len(vs) {
		return agents.Agent{}, agents.ErrNotFound
	}
	return vs[version-1], nil
}

func (s *Store) Latest(_ context.Context, tenant, id string) (agents.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vs := s.m[tenant][id]
	if len(vs) == 0 {
		return agents.Agent{}, agents.ErrNotFound
	}
	return vs[len(vs)-1], nil
}

func (s *Store) List(_ context.Context, tenant string) ([]agents.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]agents.Agent, 0, len(s.order[tenant]))
	for _, id := range s.order[tenant] {
		vs := s.m[tenant][id]
		out = append(out, vs[len(vs)-1])
	}
	return out, nil
}

func (s *Store) Versions(_ context.Context, tenant, id string) ([]agents.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vs := s.m[tenant][id]
	if len(vs) == 0 {
		return nil, agents.ErrNotFound
	}
	return append([]agents.Agent(nil), vs...), nil
}

func (s *Store) Put(_ context.Context, a agents.Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m[a.Tenant] == nil {
		s.m[a.Tenant] = map[string][]agents.Agent{}
	}
	vs := s.m[a.Tenant][a.ID]
	if a.Version != len(vs)+1 {
		return agents.ErrConflict
	}
	if len(vs) == 0 {
		s.order[a.Tenant] = append(s.order[a.Tenant], a.ID)
	}
	s.m[a.Tenant][a.ID] = append(vs, a)
	return nil
}

// Registry knows a fixed set of tools per tenant.
type Registry map[string][]string

func (r Registry) Has(_ context.Context, tenant, tool string) (bool, error) {
	for _, t := range r[tenant] {
		if t == tool {
			return true, nil
		}
	}
	return false, nil
}
