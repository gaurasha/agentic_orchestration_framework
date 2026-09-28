package executions

import (
	"context"
	"encoding/json"
	"time"
)

type Clock interface{ Now() time.Time }

type IDGen interface{ NewID() string }

// Runner runs each execution's loop as a workflow.
type Runner interface {
	Start(ctx context.Context, id string, wf Workflow) error // returns at once
	Signal(ctx context.Context, id string, s Signal) error
	Cancel(ctx context.Context, id string) error
}

type Agents interface {
	Latest(ctx context.Context, tenant, id string) (Agent, error)
	Get(ctx context.Context, tenant, id string, version int) (Agent, error)
}

type Model interface {
	Next(ctx context.Context, p Prompt) (Message, error)
}

type Gateway interface {
	Call(ctx context.Context, token, tool string, args json.RawMessage, key string) (ToolResult, error)
}

type Tokens interface {
	Issue(ctx context.Context, tenant, execution, agentID string, agentVersion int, ttl time.Duration) (string, error)
}

type Timeline interface {
	Record(ctx context.Context, e Event) error
}

type Store interface {
	Get(ctx context.Context, tenant, id string) (Execution, error) // ErrNotFound if absent
	List(ctx context.Context, tenant string) ([]Execution, error)
	Put(ctx context.Context, e Execution) error
}

type Deps struct {
	Runner   Runner
	Agents   Agents
	Model    Model
	Gateway  Gateway
	Tokens   Tokens
	Timeline Timeline
	Store    Store
	Clock    Clock
	IDs      IDGen
}
