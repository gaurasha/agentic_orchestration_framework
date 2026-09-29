// Package memory holds the in-memory Budgets counter, and fixed Catalog,
// Grants, Credentials, Placeholders and Sandbox for tests.
package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/gateway"
)

// Budgets counts calls per execution.
type Budgets struct {
	mu   sync.Mutex
	used map[string]int
}

func NewBudgets() *Budgets { return &Budgets{used: map[string]int{}} }

func (b *Budgets) Spend(_ context.Context, execution string, limit gateway.Budget) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if limit.MaxCalls > 0 && b.used[execution] >= limit.MaxCalls {
		return fmt.Errorf("%w: %d of %d calls used", gateway.ErrExhausted, b.used[execution], limit.MaxCalls)
	}
	b.used[execution]++
	return nil
}

// Catalog is a fixed set of tools per tenant.
type Catalog map[string]map[string]gateway.Tool

func (c Catalog) Tool(_ context.Context, tenant, name string) (gateway.Tool, error) {
	t, ok := c[tenant][name]
	if !ok {
		return gateway.Tool{}, fmt.Errorf("%w: tool %s not found", gateway.ErrDenied, name)
	}
	return t, nil
}

// Grants gives every caller the same grant.
type Grants gateway.Grant

func (g Grants) For(context.Context, gateway.Caller) (gateway.Grant, error) {
	return gateway.Grant(g), nil
}

// Credentials is a fixed set of values per tenant.
type Credentials map[string]map[string]string

type secret string

func (s secret) Reveal() string { return string(s) }
func (secret) String() string   { return "[secret]" }

func (c Credentials) Resolve(_ context.Context, tenant, ref string) (gateway.Secret, error) {
	v, ok := c[tenant][ref]
	if !ok {
		return nil, fmt.Errorf("credential %s not found", ref)
	}
	return secret(v), nil
}

// Placeholders issues numbered placeholders and remembers which are live.
type Placeholders struct {
	mu      sync.Mutex
	n       int
	Issued  []gateway.PlaceholderRequest
	Live    map[string]bool
	Retired []string
}

func NewPlaceholders() *Placeholders { return &Placeholders{Live: map[string]bool{}} }

func (p *Placeholders) Issue(_ context.Context, r gateway.PlaceholderRequest) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.n++
	v := fmt.Sprintf("fake_%032d", p.n)
	p.Issued = append(p.Issued, r)
	p.Live[v] = true
	return v, nil
}

func (p *Placeholders) Retire(_ context.Context, value string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.Live, value)
	p.Retired = append(p.Retired, value)
	return nil
}

// Sandbox records each command and answers with its environment, one
// KEY=value per line.
type Sandbox struct {
	mu       sync.Mutex
	Commands []gateway.Command
}

func (s *Sandbox) Exec(_ context.Context, _, _ string, cmd gateway.Command) (gateway.Output, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Commands = append(s.Commands, cmd)
	var out string
	for k, v := range cmd.Env {
		out += k + "=" + v + "\n"
	}
	return gateway.Output{Stdout: out}, nil
}

// Journal keeps claims and saved results per tenant and key.
type Journal struct {
	mu      sync.Mutex
	running map[string]bool
	saved   map[string]gateway.Result
}

func NewJournal() *Journal {
	return &Journal{running: map[string]bool{}, saved: map[string]gateway.Result{}}
}

func (j *Journal) Begin(_ context.Context, tenant, key string) (gateway.Result, bool, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	k := tenant + "/" + key
	if r, ok := j.saved[k]; ok {
		return r, true, nil
	}
	if j.running[k] {
		return gateway.Result{}, false, fmt.Errorf("%w: call %s is still running", gateway.ErrConflict, key)
	}
	j.running[k] = true
	return gateway.Result{}, false, nil
}

func (j *Journal) Finish(_ context.Context, tenant, key string, r gateway.Result) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	k := tenant + "/" + key
	delete(j.running, k)
	j.saved[k] = r
	return nil
}

func (j *Journal) Abort(_ context.Context, tenant, key string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	delete(j.running, tenant+"/"+key)
	return nil
}

// Approvals keeps approvals on file per tenant and key.
type Approvals struct {
	mu sync.Mutex
	m  map[string]bool
}

func NewApprovals() *Approvals { return &Approvals{m: map[string]bool{}} }

func (a *Approvals) Has(_ context.Context, tenant, key string) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.m[tenant+"/"+key], nil
}

func (a *Approvals) Put(_ context.Context, tenant, key string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.m[tenant+"/"+key] = true
	return nil
}
