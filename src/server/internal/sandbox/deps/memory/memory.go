// Package memory is the in-memory sandbox Store, plus a Runtime for tests
// that records each command and answers with a fixed output.
package memory

import (
	"context"
	"sync"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/sandbox"
)

type Store struct {
	mu sync.Mutex
	m  map[string]string
}

func NewStore() *Store { return &Store{m: map[string]string{}} }

func (s *Store) Get(_ context.Context, execution string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.m[execution]
	if !ok {
		return "", sandbox.ErrNotFound
	}
	return id, nil
}

func (s *Store) Put(_ context.Context, execution, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[execution] = id
	return nil
}

func (s *Store) Delete(_ context.Context, execution string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, execution)
	return nil
}

// Runtime records what it is asked to run and echoes the environment.
type Runtime struct {
	mu       sync.Mutex
	Started  []string
	Stopped  []string
	Commands []sandbox.Command
}

func (r *Runtime) Start(_ context.Context, _, execution string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Started = append(r.Started, execution)
	return "sb-" + execution, nil
}

func (r *Runtime) Exec(_ context.Context, _ string, cmd sandbox.Command) (sandbox.Output, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Commands = append(r.Commands, cmd)
	var out string
	for _, kv := range sandbox.EnvList(cmd.Env) {
		out += kv + "\n"
	}
	return sandbox.Output{Stdout: out}, nil
}

func (r *Runtime) Stop(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Stopped = append(r.Stopped, id)
	return nil
}
