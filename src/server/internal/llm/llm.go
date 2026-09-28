// Package llm is the LLM proxy, the loop's only way to a model. The default
// provider is a mock that plays a scripted conversation; a real key selects
// a real provider with no code change.
package llm

import (
	"context"
	"errors"
)

var ErrUnavailable = errors.New("model unavailable")

type API interface {
	// Complete returns the model's next message.
	Complete(ctx context.Context, r Request) (Response, error)
}
