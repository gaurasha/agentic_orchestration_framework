// Package memory is the in-memory execution Store; a restart wipes it.
package memory

import (
	"context"
	"sync"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/executions"
)

type Store struct {
	mu   sync.Mutex
	m    map[string]map[string]executions.Execution // tenant -> id
	seen map[string][]string                        // tenant -> ids by start
}

func NewStore() *Store {
	return &Store{m: map[string]map[string]executions.Execution{}, seen: map[string][]string{}}
}

func (s *Store) Get(_ context.Context, tenant, id string) (executions.Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[tenant][id]
	if !ok {
		return executions.Execution{}, executions.ErrNotFound
	}
	return e, nil
}

func (s *Store) List(_ context.Context, tenant string) ([]executions.Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := s.seen[tenant]
	out := make([]executions.Execution, 0, len(ids))
	for i := len(ids) - 1; i >= 0; i-- {
		out = append(out, s.m[tenant][ids[i]])
	}
	return out, nil
}

func (s *Store) Put(_ context.Context, e executions.Execution) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m[e.Tenant] == nil {
		s.m[e.Tenant] = map[string]executions.Execution{}
	}
	if _, ok := s.m[e.Tenant][e.ID]; !ok {
		s.seen[e.Tenant] = append(s.seen[e.Tenant], e.ID)
	}
	s.m[e.Tenant][e.ID] = e
	return nil
}
