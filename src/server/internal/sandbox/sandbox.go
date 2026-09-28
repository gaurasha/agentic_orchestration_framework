// Package sandbox runs execution tools in one sandbox per agent execution.
//
// A sandbox starts when its execution first needs one and keeps its
// working directory on a durable drive. When idle it hibernates: the drive
// is backed up and the sandbox removed. The next command restores it. Its
// only network path is the tool gateway.
//
// Needs: the interfaces in deps.go.
// Portability: interfaces only so far; no logic, service or adapters yet.
package sandbox

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound: the execution has no sandbox.
var ErrNotFound = errors.New("sandbox not found")

// API is what the sandbox manager offers.
type API interface {
	// Exec runs a command in the execution's sandbox. It starts the sandbox
	// on first use, and restores it from its backup if it was hibernated.
	Exec(ctx context.Context, tenant, execution string, cmd Command) (Output, error)
	// Hibernate backs up the execution's working directory and removes its
	// sandbox. The next Exec restores it.
	Hibernate(ctx context.Context, execution string) error
	// HibernateIdle hibernates every sandbox idle for longer than
	// Config.IdleAfter, and returns how many it hibernated.
	HibernateIdle(ctx context.Context) (int, error)
	// Release deletes the sandbox, its drive and its backups once the
	// execution has ended.
	Release(ctx context.Context, execution string) error
}

// Config is set by whoever builds the manager; the domain never reads the
// environment.
type Config struct {
	Image     string        // the sandbox image
	Gateway   string        // the tool gateway's address: the sandbox's only network path
	IdleAfter time.Duration // how long a sandbox may sit idle before it hibernates
	Limits    Limits
}

// Limits caps what one sandbox may use.
type Limits struct {
	MilliCPU    int
	MemoryBytes int64
	PIDs        int
}

// Command is a command to run.
type Command struct {
	Argv    []string
	Env     map[string]string
	Timeout time.Duration
}

// Output is what a command produced.
type Output struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// State is where a sandbox is in its life.
type State string

const (
	Running    State = "running"    // a pod is up
	Hibernated State = "hibernated" // backed up; no pod
	Released   State = "released"   // gone for good
)

// Sandbox is what the manager remembers about one execution's sandbox.
type Sandbox struct {
	Tenant       string
	Execution    string
	State        State
	Pod          string // the runtime's handle while running
	Drive        string // the durable working directory
	Snapshot     string // the latest backup, once hibernated
	LastActivity time.Time
}

// Spec is what the runtime needs to start a sandbox.
type Spec struct {
	Tenant    string
	Execution string
	Image     string
	Drive     string // mounted as the working directory
	Gateway   string // the only address the sandbox may reach
	Limits    Limits
}
