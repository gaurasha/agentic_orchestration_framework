package executions_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/executions"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/executions/deps/memory"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/executions/deps/mock"
)

type clock struct{ t time.Time }

func (c clock) Now() time.Time { return c.t }

type ids struct{ n int }

func (i *ids) NewID() string { i.n++; return fmt.Sprintf("exec-%d", i.n) }

// agents serves one agent whose version the test bumps.
type agents struct{ version int }

func (a *agents) Latest(_ context.Context, _, id string) (executions.Agent, error) {
	return a.Get(context.Background(), "", id, a.version)
}

func (a *agents) Get(_ context.Context, _, id string, version int) (executions.Agent, error) {
	return executions.Agent{ID: id, Version: version, Model: "mock", Tools: []executions.Tool{{Name: "echo"}, {Name: "deploy"}}}, nil
}

// gateway allows echo, needs an approval on file for deploy, refuses the
// rest, and remembers each caller and key. Given a store, it also notes
// how many steps the execution had saved when each call came in.
type gateway struct {
	callers  []executions.Caller
	keys     []string
	approved map[string]bool
	store    executions.Store
	saved    []int
	block    chan struct{} // an approved deploy waits on it, so a test can act mid-call
	entered  chan struct{}
}

func (g *gateway) Call(ctx context.Context, c executions.Caller, key, tool string, args json.RawMessage) (executions.ToolResult, error) {
	g.callers = append(g.callers, c)
	g.keys = append(g.keys, key)
	if g.store != nil {
		e, _ := g.store.Get(ctx, c.Tenant, c.Execution)
		g.saved = append(g.saved, len(e.Steps))
	}
	switch tool {
	case "echo":
		return executions.ToolResult{Outcome: executions.OK, Kind: "http", Output: "echoed " + string(args)}, nil
	case "deploy":
		if g.approved[key] {
			if g.block != nil {
				select {
				case g.entered <- struct{}{}:
				default:
				}
				<-g.block
			}
			return executions.ToolResult{Outcome: executions.OK, Kind: "http", Output: "deployed"}, nil
		}
		return executions.ToolResult{Outcome: executions.NeedsApproval, Output: "approval needed"}, nil
	}
	return executions.ToolResult{Outcome: executions.Refused, Output: "tool " + tool + " is not granted"}, nil
}

func (g *gateway) Approve(_ context.Context, _ executions.Caller, key string) error {
	if g.approved == nil {
		g.approved = map[string]bool{}
	}
	g.approved[key] = true
	return nil
}

// sandboxes remembers which executions released their sandbox.
type sandboxes struct{ released []string }

func (s *sandboxes) Release(_ context.Context, execution string) error {
	s.released = append(s.released, execution)
	return nil
}

func newAPI(ag *agents, gw *gateway) (executions.API, *sandboxes) {
	released := &sandboxes{}
	store := memory.NewStore()
	gw.store = store
	return executions.New(executions.Deps{
		Agents: ag, Model: mock.Model{}, Gateway: gw, Sandbox: released, Store: store,
		Clock: clock{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, IDs: &ids{},
	}, executions.Config{}), released
}

// 1.3: an execution stays on the version it started with, even after the
// agent is edited.
func TestExecutionPinsVersion(t *testing.T) {
	ag, gw := &agents{version: 2}, &gateway{}
	api, _ := newAPI(ag, gw)
	ctx := context.Background()

	e, err := api.Start(ctx, "acme", executions.Run{AgentID: "agent-1", Input: "echo {\"city\":\"London\"}\ndelete_repo {}"})
	if err != nil {
		t.Fatal(err)
	}
	ag.version = 3 // the agent is edited after the run

	got, _ := api.Get(ctx, "acme", e.ID)
	if got.AgentVersion != 2 || got.Status != executions.Succeeded {
		t.Errorf("execution = v%d %s, want v2 succeeded", got.AgentVersion, got.Status)
	}
	for _, c := range gw.callers {
		if c.AgentVersion != 2 || c.Execution != e.ID || c.Tenant != "acme" {
			t.Errorf("gateway saw caller %+v, want v2 of %s", c, e.ID)
		}
	}
	if len(got.Steps) != 2 || got.Steps[0].Outcome != executions.OK || got.Steps[1].Outcome != executions.Refused {
		t.Errorf("steps = %+v", got.Steps)
	}
	if gw.keys[0] != e.ID+":1:0" || gw.keys[1] != e.ID+":1:1" {
		t.Errorf("keys = %v, want execution:turn:index", gw.keys)
	}
	if got.Output != "Done: 1 call(s) succeeded, 1 refused or failed." {
		t.Errorf("output = %q", got.Output)
	}

	later, _ := api.Start(ctx, "acme", executions.Run{AgentID: "agent-1", Input: ""})
	if later.AgentVersion != 3 {
		t.Errorf("new execution = v%d, want v3", later.AgentVersion)
	}
	list, _ := api.List(ctx, "acme")
	if len(list) != 2 || list[0].ID != later.ID {
		t.Errorf("list = %+v, want newest first", list)
	}
	if other, _ := api.List(ctx, "globex"); len(other) != 0 {
		t.Errorf("other tenant sees %+v", other)
	}
}

// 4.1 and 4.2: a call that needs approval pauses the execution and nothing
// runs until someone decides; approval runs the call, and the calls after
// it; rejection returns the reason to the model and the execution goes on.
func TestApprovalPausesAndResumes(t *testing.T) {
	ctx := context.Background()
	for _, approve := range []bool{true, false} {
		gw := &gateway{}
		api, _ := newAPI(&agents{version: 1}, gw)
		e, err := api.Start(ctx, "acme", executions.Run{AgentID: "a", Input: "deploy {\"env\":\"prod\"}\necho {}"})
		if err != nil {
			t.Fatal(err)
		}
		if e.Status != executions.Waiting || e.Wait == nil || e.Wait.Kind != executions.WaitApproval || e.Wait.Tool != "deploy" || len(e.Steps) != 0 {
			t.Fatalf("after start: %+v", e)
		}
		if len(gw.keys) != 1 { // deploy was asked once; echo did not run
			t.Fatalf("gateway calls before the decision: %v", gw.keys)
		}
		if _, err := api.Answer(ctx, "acme", e.ID, executions.Reply{Key: e.Wait.Key, Text: "yes"}); !errors.Is(err, executions.ErrState) {
			t.Errorf("an answer is not an approval: err = %v", err)
		}

		e, err = api.Decide(ctx, "acme", e.ID, executions.Decision{Key: e.Wait.Key, Approve: approve, Reason: "not today"})
		if err != nil {
			t.Fatal(err)
		}
		if e.Status != executions.Succeeded || len(e.Steps) != 2 || e.Wait != nil {
			t.Fatalf("approve=%v: after decision: %+v", approve, e)
		}
		first, second := e.Steps[0], e.Steps[1]
		if approve && (first.Outcome != executions.OK || !first.Approved || first.Output != "deployed") {
			t.Errorf("approved step = %+v", first)
		}
		if !approve && (first.Outcome != executions.Rejected || first.Output != "not today" || len(gw.keys) != 2) {
			t.Errorf("rejected step = %+v, gateway calls %v", first, gw.keys)
		}
		if second.Tool != "echo" || second.Outcome != executions.OK {
			t.Errorf("the call after the decision = %+v", second)
		}
		if _, err := api.Decide(ctx, "acme", e.ID, executions.Decision{Key: "gone", Approve: true}); !errors.Is(err, executions.ErrState) {
			t.Errorf("a second decision: err = %v, want ErrState", err)
		}
	}
}

// 4.3: an execution that asks a question resumes with the answer.
func TestQuestionWaitsForAnswer(t *testing.T) {
	ctx := context.Background()
	api, _ := newAPI(&agents{version: 1}, &gateway{})
	e, err := api.Start(ctx, "acme", executions.Run{AgentID: "a", Input: "echo {}\nask Which region?\necho {\"r\":\"eu\"}"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Status != executions.Waiting || e.Wait.Kind != executions.WaitQuestion || e.Wait.Question != "Which region?" || len(e.Steps) != 1 {
		t.Fatalf("after start: %+v", e)
	}
	if _, err := api.Decide(ctx, "acme", e.ID, executions.Decision{Key: e.Wait.Key, Approve: true}); !errors.Is(err, executions.ErrState) {
		t.Errorf("a decision is not an answer: err = %v", err)
	}
	e, err = api.Answer(ctx, "acme", e.ID, executions.Reply{Key: e.Wait.Key, Text: "eu"})
	if err != nil || e.Status != executions.Succeeded || len(e.Steps) != 3 {
		t.Fatalf("after answer: %+v, %v", e, err)
	}
	if q := e.Steps[1]; q.Kind != "question" || !strings.Contains(q.Output, "Q: Which region?\nA: eu") {
		t.Errorf("question step = %+v", q)
	}
	if !strings.Contains(e.Output, "You answered 1 question(s).") {
		t.Errorf("output = %q", e.Output)
	}
}

// 4.4: a waiting execution can be cancelled; it ends as cancelled and the
// pending call never runs.
func TestCancelWaiting(t *testing.T) {
	ctx := context.Background()
	gw := &gateway{}
	api, released := newAPI(&agents{version: 1}, gw)
	e, _ := api.Start(ctx, "acme", executions.Run{AgentID: "a", Input: "deploy {}\necho {}"})
	if slices.Contains(released.released, e.ID) {
		t.Errorf("the sandbox was released while the execution waits")
	}
	e, err := api.Cancel(ctx, "acme", e.ID, "changed my mind")
	if err != nil || e.Status != executions.Cancelled || e.Wait != nil || e.Error != "cancelled: changed my mind" || e.EndedAt.IsZero() {
		t.Fatalf("after cancel: %+v, %v", e, err)
	}
	if !slices.Contains(released.released, e.ID) {
		t.Errorf("the sandbox was not released at the end")
	}
	if _, err := api.Decide(ctx, "acme", e.ID, executions.Decision{Key: "gone", Approve: true}); !errors.Is(err, executions.ErrState) {
		t.Errorf("decide after cancel: err = %v, want ErrState", err)
	}
	if len(gw.keys) != 1 || gw.approved["deploy"] {
		t.Errorf("the pending call ran: %v", gw.keys)
	}
	done, _ := api.Start(ctx, "acme", executions.Run{AgentID: "a", Input: "echo {}"})
	if _, err := api.Cancel(ctx, "acme", done.ID, "x"); !errors.Is(err, executions.ErrState) {
		t.Errorf("cancel a finished execution: err = %v, want ErrState", err)
	}
}

// 3.4 at the loop: a retried step calls the gateway with the same key.
func TestRetryUsesTheSameKey(t *testing.T) {
	ctx := context.Background()
	gw := &gateway{}
	api, _ := newAPI(&agents{version: 1}, gw)
	e, _ := api.Start(ctx, "acme", executions.Run{AgentID: "a", Input: "echo {\"x\":1}"})
	e, err := api.Retry(ctx, "acme", e.ID, 1)
	if err != nil || len(e.Steps) != 2 || gw.keys[1] != gw.keys[0] || e.Steps[1].Tool != "echo" {
		t.Fatalf("retry = %+v, %v, keys %v", e, err, gw.keys)
	}
	if _, err := api.Retry(ctx, "acme", e.ID, 9); !errors.Is(err, executions.ErrInvalid) {
		t.Errorf("bad step: err = %v", err)
	}
}

// 6.1: the same call repeated is refused with a message to the model.
func TestRepeatedCallIsRefused(t *testing.T) {
	ctx := context.Background()
	gw := &gateway{}
	api, _ := newAPI(&agents{version: 1}, gw)
	line := "echo {\"city\":\"London\"}\n"
	e, _ := api.Start(ctx, "acme", executions.Run{AgentID: "a", Input: strings.Repeat(line, 5)})
	if len(e.Steps) != 5 || len(gw.keys) != 3 {
		t.Fatalf("steps %d, gateway calls %d; want 5 and 3", len(e.Steps), len(gw.keys))
	}
	for _, s := range e.Steps[3:] {
		if s.Outcome != executions.Refused || !strings.Contains(s.Output, "already made this exact call 3 times") {
			t.Errorf("repeat step = %+v", s)
		}
	}
}

// 7.1: a running execution shows each step as soon as it completes: when
// the third call comes in, the store already holds the first two steps.
func TestStepsAppearAsTheyComplete(t *testing.T) {
	gw := &gateway{}
	api, _ := newAPI(&agents{version: 1}, gw)
	if _, err := api.Start(context.Background(), "acme", executions.Run{AgentID: "agent-1", Input: "echo {}\necho {}\necho {}"}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(gw.saved, []int{0, 1, 2}) {
		t.Errorf("steps saved before each call = %v, want [0 1 2]", gw.saved)
	}
}

// 6.3: transitions of one execution never interleave. A cancel that
// arrives while an approved call is in flight waits for it, then is
// refused, and the finished run is what stays saved.
func TestDecideAndCancelCannotBothWin(t *testing.T) {
	gw := &gateway{block: make(chan struct{}), entered: make(chan struct{}, 1)}
	api, _ := newAPI(&agents{version: 1}, gw)
	ctx := context.Background()
	e, err := api.Start(ctx, "acme", executions.Run{AgentID: "a", Input: "deploy {}\necho {}"})
	if err != nil || e.Status != executions.Waiting {
		t.Fatalf("start = %+v, %v", e, err)
	}
	decided := make(chan executions.Execution, 1)
	go func() {
		out, _ := api.Decide(ctx, "acme", e.ID, executions.Decision{Key: e.Wait.Key, Approve: true})
		decided <- out
	}()
	<-gw.entered // the approved deploy is in flight
	cancelled := make(chan error, 1)
	go func() {
		_, err := api.Cancel(ctx, "acme", e.ID, "too late")
		cancelled <- err
	}()
	select {
	case err := <-cancelled:
		t.Fatalf("cancel returned (%v) while the decision was in flight", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(gw.block)
	out := <-decided
	if err := <-cancelled; !errors.Is(err, executions.ErrState) {
		t.Errorf("cancel after the decision: err = %v, want ErrState", err)
	}
	got, _ := api.Get(ctx, "acme", e.ID)
	if out.Status != executions.Succeeded || got.Status != executions.Succeeded || len(got.Steps) != 2 {
		t.Errorf("decided = %s, saved = %s with %d steps; want succeeded with 2 steps", out.Status, got.Status, len(got.Steps))
	}
	if _, err := api.Retry(ctx, "acme", e.ID, 1); err != nil {
		t.Errorf("retry after the run: %v", err)
	}
}

// 4.2: a decision names the wait it is for. Two approvals for the same
// call, one after the other, do not approve two calls: the second finds
// its wait gone and is refused, while the next wait stays untouched. A
// decision with no key is invalid.
func TestStaleDecisionIsRefused(t *testing.T) {
	ctx := context.Background()
	gw := &gateway{}
	api, _ := newAPI(&agents{version: 1}, gw)
	e, err := api.Start(ctx, "acme", executions.Run{AgentID: "a", Input: "deploy {\"n\":1}\ndeploy {\"n\":2}"})
	if err != nil || e.Status != executions.Waiting {
		t.Fatalf("start = %+v, %v", e, err)
	}
	first := e.Wait.Key
	if _, err := api.Decide(ctx, "acme", e.ID, executions.Decision{Approve: true}); !errors.Is(err, executions.ErrInvalid) {
		t.Errorf("a decision without a key: err = %v, want ErrInvalid", err)
	}
	e, err = api.Decide(ctx, "acme", e.ID, executions.Decision{Key: first, Approve: true})
	if err != nil || e.Status != executions.Waiting || e.Wait.Key == first || len(e.Steps) != 1 {
		t.Fatalf("after the first approval: %+v, %v; want waiting for the second deploy", e, err)
	}
	// The same approval again, as a second client that saw only the first wait would send it.
	if _, err := api.Decide(ctx, "acme", e.ID, executions.Decision{Key: first, Approve: true}); !errors.Is(err, executions.ErrState) {
		t.Errorf("the stale approval: err = %v, want ErrState", err)
	}
	got, _ := api.Get(ctx, "acme", e.ID)
	if got.Status != executions.Waiting || got.Wait == nil || got.Wait.Key != e.Wait.Key || len(got.Steps) != 1 {
		t.Fatalf("the stale approval changed the run: %+v", got)
	}
	if len(gw.approved) != 1 {
		t.Errorf("approvals on file: %v, want only the first", gw.approved)
	}
	e, err = api.Decide(ctx, "acme", e.ID, executions.Decision{Key: e.Wait.Key, Approve: false, Reason: "no"})
	if err != nil || e.Status != executions.Succeeded || len(e.Steps) != 2 || e.Steps[1].Outcome != executions.Rejected {
		t.Errorf("the decision for the second wait: %+v, %v", e, err)
	}
}
