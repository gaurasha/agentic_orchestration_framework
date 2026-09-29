// Package executions runs agents. Starting one pins the agent's latest
// version, then the loop calls the model, runs the tools it asks for through
// the gateway, feeds back each result and stops at the final answer. Every
// tool call is kept as a step of the execution.
//
// The loop pauses for a human when a call needs approval or the model asks
// a question, and resumes inside the request that decides or answers. A
// waiting execution can be cancelled; its pending call never runs. Each
// call carries a key, execution:turn:index, so a retry gets the saved
// result instead of running twice.
package executions

import (
	"context"
	"errors"
)

var (
	ErrNotFound = errors.New("execution not found")
	ErrInvalid  = errors.New("invalid execution") // e.g. no agent named
	ErrState    = errors.New("wrong state")       // e.g. a decision when nothing waits
)

type API interface {
	// Start pins the agent's latest version and runs the loop until it
	// ends or waits on a human.
	Start(ctx context.Context, tenant string, r Run) (Execution, error)
	Get(ctx context.Context, tenant, id string) (Execution, error)
	// List returns the tenant's executions, newest first.
	List(ctx context.Context, tenant string) ([]Execution, error)
	// Decide answers a pending approval and runs on. A rejection's reason
	// goes to the model. The decision names the wait (Wait.Key); one for a
	// wait that has passed is ErrState, never applied to the next.
	Decide(ctx context.Context, tenant, id string, d Decision) (Execution, error)
	// Answer answers a pending question, named by its key, and runs on. It
	// is never an approval.
	Answer(ctx context.Context, tenant, id string, r Reply) (Execution, error)
	// Cancel ends a waiting execution; the pending call never runs.
	Cancel(ctx context.Context, tenant, id, reason string) (Execution, error)
	// Retry repeats a step's call with its key. The gateway returns the
	// saved result, so nothing runs twice; the step is added as replayed.
	Retry(ctx context.Context, tenant, id string, step int) (Execution, error)
}

func New(d Deps, cfg Config) API {
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = 10
	}
	if cfg.MaxRepeats <= 0 {
		cfg.MaxRepeats = 3
	}
	return &service{d: d, cfg: cfg}
}
