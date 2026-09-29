package local

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/sandbox"
)

// 5.1: a command sees only the environment it is given, plus the platform
// CA to trust.
func TestEnvIsOnlyWhatIsGiven(t *testing.T) {
	t.Setenv("LEAKED_FROM_HOST", "host-value")
	ctx := context.Background()
	r := Runtime{CAFile: "/etc/egress-ca.pem"}
	id, err := r.Start(ctx, "acme", "e1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Stop(ctx, id) })

	out, err := r.Exec(ctx, id, sandbox.Command{Argv: []string{"sh", "-c", "echo $GH_TOKEN; env"}, Env: map[string]string{"GH_TOKEN": "fake_abc"}, Timeout: 5 * time.Second})
	if err != nil || out.ExitCode != 0 {
		t.Fatalf("Exec = %+v, %v", out, err)
	}
	if !strings.HasPrefix(out.Stdout, "fake_abc\n") {
		t.Errorf("echo printed %q", out.Stdout)
	}
	if strings.Contains(out.Stdout, "LEAKED_FROM_HOST") {
		t.Errorf("the host's environment leaked in:\n%s", out.Stdout)
	}
	if !strings.Contains(out.Stdout, "SSL_CERT_FILE=/etc/egress-ca.pem\n") {
		t.Errorf("the platform CA is not named:\n%s", out.Stdout)
	}
}

func TestExitCodeAndTimeout(t *testing.T) {
	ctx := context.Background()
	r := Runtime{}
	id, _ := r.Start(ctx, "acme", "e2")
	t.Cleanup(func() { _ = r.Stop(ctx, id) })

	out, err := r.Exec(ctx, id, sandbox.Command{Argv: []string{"sh", "-c", "echo oops >&2; exit 3"}})
	if err != nil || out.ExitCode != 3 || !strings.Contains(out.Stderr, "oops") {
		t.Errorf("exit: got %+v, %v", out, err)
	}
	_, err = r.Exec(ctx, id, sandbox.Command{Argv: []string{"sleep", "5"}, Timeout: 50 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("timeout: err = %v, want DeadlineExceeded", err)
	}
}

// 6.4: the timeout kills the shell's children too, and Exec returns at once
// instead of waiting for them to let go of the output pipes.
func TestTimeoutKillsTheProcessGroup(t *testing.T) {
	ctx := context.Background()
	r := Runtime{}
	id, _ := r.Start(ctx, "acme", "e3")
	t.Cleanup(func() { _ = r.Stop(ctx, id) })
	start := time.Now()
	_, err := r.Exec(ctx, id, sandbox.Command{Argv: []string{"sh", "-c", "(sleep 0.4; echo late > marker) & sleep 5"}, Timeout: 50 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
	if time.Since(start) > 1500*time.Millisecond {
		t.Errorf("Exec took %s: it waited on the shell's children", time.Since(start))
	}
	time.Sleep(600 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(id, "marker")); err == nil {
		t.Errorf("a child of the shell outlived the timeout and wrote the marker")
	}
	// Output is capped while it is collected.
	out, err := r.Exec(ctx, id, sandbox.Command{Argv: []string{"sh", "-c", "yes | head -c 300000"}, Timeout: 5 * time.Second})
	if err != nil || len(out.Stdout) > sandbox.MaxOutput+20 || !strings.HasSuffix(out.Stdout, "…[truncated]") {
		t.Errorf("noisy command: %d bytes, err %v", len(out.Stdout), err)
	}
}
