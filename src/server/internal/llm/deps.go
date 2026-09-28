package llm

import "context"

// Provider is a model API: the scripted mock or a real one.
type Provider interface {
	Complete(ctx context.Context, r Request) (Response, error)
}

type Deps struct {
	Provider Provider
}
