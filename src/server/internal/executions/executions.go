// Package executions runs agents. The loop calls the model, runs the tools
// it asks for through the gateway, feeds back the results, and stops at the
// final answer. A tool that needs approval, or a question from the model,
// pauses the execution until a human responds.
//
// The loop depends only on Runner: in memory for now, so a restart loses
// running executions; Temporal can replace it.
package executions

import (
	"context"
	"errors"
)

var (
	ErrNotFound = errors.New("execution not found")
	ErrState    = errors.New("wrong state") // e.g. a decision when nothing waits
)

type API interface {
	// Start pins the agent's latest version and returns at once; the loop
	// runs in the background.
	Start(ctx context.Context, tenant, agentID, input string) (Execution, error)
	Get(ctx context.Context, tenant, id string) (Execution, error)
	List(ctx context.Context, tenant string) ([]Execution, error)
	// Cancel stops the execution at its next step.
	Cancel(ctx context.Context, tenant, id, reason string) error
	// Decide answers a pending approval; a rejection's reason goes to the model.
	Decide(ctx context.Context, tenant, id string, d Decision) error
	// Answer answers a pending question.
	Answer(ctx context.Context, tenant, id, text string) error
}
