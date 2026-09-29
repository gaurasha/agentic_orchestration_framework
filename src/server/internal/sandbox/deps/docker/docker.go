// Package docker runs each sandbox as a Docker volume for its files and one
// container per command, with only the environment the command is given,
// plus the platform CA mounted read-only and named as SSL_CERT_FILE, so
// the command's TLS connections through egress are trusted. The container
// reaches the host as host.docker.internal.
package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/sandbox"
)

type Runtime struct {
	Image  string // built by `make sandbox-image`
	CAFile string // the platform CA's certificate on this machine; empty for none
}

// caPath is where the CA is mounted in the container.
const caPath = "/etc/egress-ca.pem"

func (r Runtime) Start(ctx context.Context, tenant, execution string) (string, error) {
	name := "sandbox-" + tenant + "-" + execution
	if out, err := exec.CommandContext(ctx, "docker", "volume", "create", name).CombinedOutput(); err != nil {
		return "", fmt.Errorf("docker volume create: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return name, nil
}

func (r Runtime) Exec(ctx context.Context, id string, cmd sandbox.Command) (sandbox.Output, error) {
	if cmd.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cmd.Timeout)
		defer cancel()
	}
	// Named, so a container the timeout leaves behind can be removed: killing
	// the docker client does not stop the container.
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	name := id + "-" + hex.EncodeToString(suffix)
	args := []string{"run", "--rm", "--name", name, "--add-host=host.docker.internal:host-gateway", "-v", id + ":/work", "-w", "/work"}
	if r.CAFile != "" {
		args = append(args, "-v", r.CAFile+":"+caPath+":ro", "-e", "SSL_CERT_FILE="+caPath)
	}
	for _, kv := range sandbox.EnvList(cmd.Env) {
		args = append(args, "-e", kv)
	}
	args = append(append(args, r.Image), cmd.Argv...)
	c := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr sandbox.Capped
	c.Stdout, c.Stderr = &stdout, &stderr
	err := c.Run()
	out := sandbox.Output{Stdout: stdout.String(), Stderr: stderr.String()}
	var exit *exec.ExitError
	switch {
	case err != nil && ctx.Err() != nil: // the client was killed at the timeout; the container was not
		rm, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = exec.CommandContext(rm, "docker", "rm", "-f", name).Run()
		return out, fmt.Errorf("docker run: %w", ctx.Err())
	case errors.As(err, &exit):
		out.ExitCode = exit.ExitCode()
	case err != nil:
		return out, fmt.Errorf("docker run: %w", err)
	}
	return out, nil
}

func (r Runtime) Stop(ctx context.Context, id string) error {
	if out, err := exec.CommandContext(ctx, "docker", "volume", "rm", id).CombinedOutput(); err != nil {
		return fmt.Errorf("docker volume rm: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
