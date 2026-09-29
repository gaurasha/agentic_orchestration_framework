// Package local runs each sandbox as processes on this machine, in a
// temporary directory, with no isolation beyond the environment: a command
// sees only the variables it is given, plus PATH, HOME and, as
// SSL_CERT_FILE, the platform CA to trust for egress.
package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/sandbox"
)

type Runtime struct {
	Path   string // for the command's PATH; empty means this process's
	CAFile string // the platform CA's certificate, given as SSL_CERT_FILE; empty for none
}

func (r Runtime) Start(_ context.Context, tenant, execution string) (string, error) {
	return os.MkdirTemp("", "sandbox-"+tenant+"-"+execution+"-")
}

func (r Runtime) Exec(ctx context.Context, id string, cmd sandbox.Command) (sandbox.Output, error) {
	if cmd.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cmd.Timeout)
		defer cancel()
	}
	c := exec.CommandContext(ctx, cmd.Argv[0], cmd.Argv[1:]...)
	c.Dir = id
	// The command gets its own process group, and the timeout kills the
	// whole group: a shell's children would otherwise run on and hold the
	// output pipes open. WaitDelay bounds the wait for those pipes.
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error { return syscall.Kill(-c.Process.Pid, syscall.SIGKILL) }
	c.WaitDelay = time.Second
	path := r.Path
	if path == "" {
		path = os.Getenv("PATH")
	}
	c.Env = append(sandbox.EnvList(cmd.Env), "PATH="+path, "HOME="+id)
	if r.CAFile != "" {
		c.Env = append(c.Env, "SSL_CERT_FILE="+r.CAFile)
	}
	var stdout, stderr sandbox.Capped
	c.Stdout, c.Stderr = &stdout, &stderr
	err := c.Run()
	out := sandbox.Output{Stdout: stdout.String(), Stderr: stderr.String()}
	var exit *exec.ExitError
	switch {
	case err != nil && ctx.Err() != nil: // killed at the timeout
		return out, fmt.Errorf("%s: %w", cmd.Argv[0], ctx.Err())
	case errors.As(err, &exit):
		out.ExitCode = exit.ExitCode()
	case err != nil:
		return out, fmt.Errorf("%s: %w", cmd.Argv[0], err)
	}
	return out, nil
}

func (r Runtime) Stop(_ context.Context, id string) error {
	return os.RemoveAll(id)
}
