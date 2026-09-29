package executions

import (
	"context"
	"encoding/json"
	"time"
)

type Clock interface{ Now() time.Time }

type IDGen interface{ NewID() string }

type Agents interface {
	Latest(ctx context.Context, tenant, id string) (Agent, error)
	Get(ctx context.Context, tenant, id string, version int) (Agent, error)
}

// Model is the LLM: the scripted mock for now.
type Model interface {
	Next(ctx context.Context, p Prompt) (Message, error)
}

// Gateway runs one tool call. A refusal, or a call that needs approval,
// comes back as a result, not an error, so the loop can act on it.
type Gateway interface {
	Call(ctx context.Context, c Caller, key, tool string, args json.RawMessage) (ToolResult, error)
	// Approve puts a human's approval on file for the call with this key.
	Approve(ctx context.Context, c Caller, key string) error
}

// Sandbox releases an execution's sandbox once the execution ends.
type Sandbox interface {
	Release(ctx context.Context, execution string) error
}

type Store interface {
	Get(ctx context.Context, tenant, id string) (Execution, error) // ErrNotFound if absent
	List(ctx context.Context, tenant string) ([]Execution, error)  // newest first
	Put(ctx context.Context, e Execution) error
}

type Deps struct {
	Agents  Agents
	Model   Model
	Gateway Gateway
	Sandbox Sandbox // optional
	Store   Store
	Clock   Clock
	IDs     IDGen
}
