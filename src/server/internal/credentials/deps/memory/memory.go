// Package memory holds the in-memory Secrets and Placeholders stores; a
// restart wipes them.
package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/credentials"
)

type Secrets struct {
	mu sync.Mutex
	m  map[string]map[string]credentials.Secret // tenant -> ref -> value
}

func NewSecrets() *Secrets { return &Secrets{m: map[string]map[string]credentials.Secret{}} }

func (s *Secrets) Get(_ context.Context, tenant, ref string) (credentials.Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sec, ok := s.m[tenant][ref]
	if !ok {
		return credentials.Secret{}, credentials.ErrNotFound
	}
	return sec, nil
}

func (s *Secrets) Put(_ context.Context, tenant, ref string, sec credentials.Secret) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m[tenant] == nil {
		s.m[tenant] = map[string]credentials.Secret{}
	}
	s.m[tenant][ref] = sec
	return nil
}

func (s *Secrets) List(_ context.Context, tenant string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	refs := make([]string, 0, len(s.m[tenant]))
	for ref := range s.m[tenant] {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	return refs, nil
}

type Placeholders struct {
	mu sync.Mutex
	m  map[string]credentials.Placeholder
}

func NewPlaceholders() *Placeholders { return &Placeholders{m: map[string]credentials.Placeholder{}} }

func (p *Placeholders) Get(_ context.Context, value string) (credentials.Placeholder, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	ph, ok := p.m[value]
	if !ok {
		return credentials.Placeholder{}, credentials.ErrUnauthenticated
	}
	return ph, nil
}

func (p *Placeholders) Put(_ context.Context, value string, ph credentials.Placeholder) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.m[value] = ph
	return nil
}

func (p *Placeholders) Delete(_ context.Context, value string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.m, value)
	return nil
}
