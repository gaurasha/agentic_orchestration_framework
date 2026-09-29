// Package memory is the in-memory tool Store; a restart wipes it.
package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/tools"
)

type Store struct {
	mu sync.Mutex
	m  map[string]map[string]tools.Tool // tenant -> name -> tool
}

func NewStore() *Store { return &Store{m: map[string]map[string]tools.Tool{}} }

func (s *Store) Get(_ context.Context, tenant, name string) (tools.Tool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.m[tenant][name]
	if !ok {
		return tools.Tool{}, tools.ErrNotFound
	}
	return t, nil
}

func (s *Store) Put(_ context.Context, t tools.Tool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m[t.Tenant] == nil {
		s.m[t.Tenant] = map[string]tools.Tool{}
	}
	s.m[t.Tenant][t.Name] = t
	return nil
}

func (s *Store) List(_ context.Context, tenant string) ([]tools.Tool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]tools.Tool, 0, len(s.m[tenant]))
	for _, t := range s.m[tenant] {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
