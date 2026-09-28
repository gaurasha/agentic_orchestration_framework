// Package gateway is the one path every tool call takes.
// For each call it verifies the caller's token, loads the tool, checks the
// grant, the argument allowlist and the budget, claims the idempotency key
// so a retry never runs twice, and runs the tool, adding the real
// credential only to the external request.
package gateway

import (
	"context"
	"errors"
)

// A refused call wraps one of these, with the reason in its message.
var (
	ErrUnauthenticated = errors.New("unauthenticated")  // token missing, forged or expired
	ErrDenied          = errors.New("denied")           // tool, argument or host not allowed
	ErrExhausted       = errors.New("budget exhausted") // the execution's budget is spent
	ErrConflict        = errors.New("conflict")         // the same key is still running
)

type API interface {
	Call(ctx context.Context, c Call) (Result, error)
}
