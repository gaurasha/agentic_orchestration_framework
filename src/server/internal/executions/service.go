package executions

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type service struct {
	d   Deps
	cfg Config

	mu    sync.Mutex
	locks map[string]*sync.Mutex // one per execution: its transitions never interleave
}

// lock serializes everything that reads, changes and saves one execution,
// so a decision and a cancel cannot both win, and a save cannot lose a
// step another request appended. The caller defers the returned unlock.
func (s *service) lock(tenant, id string) func() {
	s.mu.Lock()
	if s.locks == nil {
		s.locks = map[string]*sync.Mutex{}
	}
	l, ok := s.locks[tenant+"/"+id]
	if !ok {
		l = &sync.Mutex{}
		s.locks[tenant+"/"+id] = l
	}
	s.mu.Unlock()
	l.Lock()
	return l.Unlock
}

func (s *service) Start(ctx context.Context, tenant string, r Run) (Execution, error) {
	if r.AgentID == "" {
		return Execution{}, fmt.Errorf("%w: no agent", ErrInvalid)
	}
	agent, err := s.d.Agents.Latest(ctx, tenant, r.AgentID)
	if err != nil {
		return Execution{}, err
	}
	id := s.d.IDs.NewID()
	defer s.lock(tenant, id)()
	e := Execution{
		ID: id, Tenant: tenant, AgentID: agent.ID, AgentVersion: agent.Version,
		Input: r.Input, Description: r.Description, Status: Running, Steps: []Step{}, StartedAt: s.d.Clock.Now(),
		History: []Message{{Role: "user", Text: r.Input}},
	}
	if err := s.d.Store.Put(ctx, e); err != nil {
		return Execution{}, fmt.Errorf("start execution: %w", err)
	}
	return s.finish(ctx, s.loop(ctx, e, agent))
}

func (s *service) Decide(ctx context.Context, tenant, id string, d Decision) (Execution, error) {
	defer s.lock(tenant, id)()
	e, agent, err := s.waiting(ctx, tenant, id, WaitApproval, d.Key)
	if err != nil {
		return Execution{}, err
	}
	caller := s.caller(e)
	calls := pendingCalls(e.History)
	w := *e.Wait
	e.Wait, e.Status = nil, Running
	call := calls[w.Index]
	key := callKey(e.ID, w.Turn, w.Index)
	var r ToolResult
	if d.Approve {
		if err := s.d.Gateway.Approve(ctx, caller, key); err != nil {
			return Execution{}, fmt.Errorf("approve %s: %w", key, err)
		}
		r = s.call(ctx, caller, key, call)
		if r.Outcome == NeedsApproval { // the gateway did not see the approval; a wiring bug
			return Execution{}, fmt.Errorf("%w: the gateway still wants approval for %s", ErrState, key)
		}
	} else {
		r = ToolResult{Outcome: Rejected, Output: d.Reason}
	}
	step := stepOf(len(e.Steps)+1, call, key, r, s.d.Clock.Now())
	step.Approved = d.Approve
	e.Steps = append(e.Steps, step)
	e.History = append(e.History, toolMessage(call, r))
	if s.runCalls(ctx, &e, caller, calls, w.Turn, w.Index+1) {
		return s.finish(ctx, e)
	}
	return s.finish(ctx, s.loop(ctx, e, agent))
}

func (s *service) Answer(ctx context.Context, tenant, id string, r Reply) (Execution, error) {
	defer s.lock(tenant, id)()
	e, agent, err := s.waiting(ctx, tenant, id, WaitQuestion, r.Key)
	if err != nil {
		return Execution{}, err
	}
	e.Steps = append(e.Steps, questionStep(len(e.Steps)+1, e.Wait.Question, r.Text, s.d.Clock.Now()))
	e.History = append(e.History, Message{Role: "user", Text: r.Text})
	e.Wait, e.Status = nil, Running
	return s.finish(ctx, s.loop(ctx, e, agent))
}

func (s *service) Cancel(ctx context.Context, tenant, id, reason string) (Execution, error) {
	defer s.lock(tenant, id)()
	e, err := s.d.Store.Get(ctx, tenant, id)
	if err != nil {
		return Execution{}, err
	}
	if e.Status != Waiting {
		return Execution{}, fmt.Errorf("%w: execution is %s, only a waiting one can be cancelled", ErrState, e.Status)
	}
	e.Wait, e.Status, e.Error = nil, Cancelled, "cancelled: "+reason
	return s.finish(ctx, e)
}

func (s *service) Retry(ctx context.Context, tenant, id string, step int) (Execution, error) {
	defer s.lock(tenant, id)()
	e, err := s.d.Store.Get(ctx, tenant, id)
	if err != nil {
		return Execution{}, err
	}
	if e.Status == Running {
		return Execution{}, fmt.Errorf("%w: execution is running", ErrState)
	}
	if step < 1 || step > len(e.Steps) {
		return Execution{}, fmt.Errorf("%w: no step %d", ErrInvalid, step)
	}
	prev := e.Steps[step-1]
	if prev.Key == "" {
		return Execution{}, fmt.Errorf("%w: step %d was not a tool call", ErrInvalid, step)
	}
	r := s.call(ctx, s.caller(e), prev.Key, ToolCall{Name: prev.Tool, Args: prev.Args})
	if r.Outcome == NeedsApproval {
		r = ToolResult{Outcome: Refused, Output: "approval needed"}
	}
	e.Steps = append(e.Steps, stepOf(len(e.Steps)+1, ToolCall{Name: prev.Tool, Args: prev.Args}, prev.Key, r, s.d.Clock.Now()))
	if err := s.d.Store.Put(ctx, e); err != nil {
		return Execution{}, fmt.Errorf("retry step %d: %w", step, err)
	}
	return e, nil
}

func (s *service) Get(ctx context.Context, tenant, id string) (Execution, error) {
	return s.d.Store.Get(ctx, tenant, id)
}

func (s *service) List(ctx context.Context, tenant string) ([]Execution, error) {
	return s.d.Store.List(ctx, tenant)
}

// waiting loads an execution that waits for the given kind of answer, and
// the agent version it is pinned to.
// waiting loads an execution that waits for kind, under the wait named by
// key. Two humans answering the same wait are serialized by the lock; the
// second finds the key gone and is refused, so a decision never lands on
// a wait its author did not see.
func (s *service) waiting(ctx context.Context, tenant, id string, kind WaitKind, key string) (Execution, Agent, error) {
	if key == "" {
		return Execution{}, Agent{}, fmt.Errorf("%w: a %s must name the wait it is for (key)", ErrInvalid, kind)
	}
	e, err := s.d.Store.Get(ctx, tenant, id)
	if err != nil {
		return Execution{}, Agent{}, err
	}
	if e.Status != Waiting || e.Wait == nil || e.Wait.Kind != kind {
		return Execution{}, Agent{}, fmt.Errorf("%w: execution is not waiting for a %s", ErrState, kind)
	}
	if e.Wait.Key != key {
		return Execution{}, Agent{}, fmt.Errorf("%w: wait %s has passed; the execution now waits for %s", ErrState, key, e.Wait.Key)
	}
	agent, err := s.d.Agents.Get(ctx, tenant, e.AgentID, e.AgentVersion)
	if err != nil {
		return Execution{}, Agent{}, fmt.Errorf("agent %s v%d: %w", e.AgentID, e.AgentVersion, err)
	}
	return e, agent, nil
}

// loop runs the model until its final answer, a wait, or MaxSteps.
func (s *service) loop(ctx context.Context, e Execution, agent Agent) Execution {
	caller := s.caller(e)
	for turns(e.History) < s.cfg.MaxSteps {
		msg, err := s.d.Model.Next(ctx, Prompt{Model: agent.Model, System: agent.Instructions, Tools: agent.Tools, History: e.History})
		if err != nil {
			e.Status, e.Error = Failed, fmt.Sprintf("model: %v", err)
			return e
		}
		e.History = append(e.History, msg)
		if msg.Question != "" {
			e.Status = Waiting
			e.Wait = &Wait{Key: questionKey(e.ID, turns(e.History)), Kind: WaitQuestion, Question: msg.Question, AskedAt: s.d.Clock.Now()}
			return e
		}
		if len(msg.ToolCalls) == 0 {
			e.Status, e.Output = Succeeded, msg.Text
			return e
		}
		if s.runCalls(ctx, &e, caller, msg.ToolCalls, turns(e.History), 0) {
			return e
		}
	}
	e.Status, e.Error = Failed, fmt.Sprintf("no final answer after %d model calls", s.cfg.MaxSteps)
	return e
}

// runCalls runs calls[from:] and reports whether it paused for approval.
func (s *service) runCalls(ctx context.Context, e *Execution, caller Caller, calls []ToolCall, turn, from int) (paused bool) {
	for i := from; i < len(calls); i++ {
		call := calls[i]
		key := callKey(e.ID, turn, i)
		var r ToolResult
		if repeating(e.Steps, call, s.cfg.MaxRepeats) {
			r = ToolResult{Outcome: Refused, Output: fmt.Sprintf("you have already made this exact call %d times; it was not run again", s.cfg.MaxRepeats)}
		} else {
			r = s.call(ctx, caller, key, call)
		}
		if r.Outcome == NeedsApproval {
			e.Status = Waiting
			e.Wait = &Wait{Key: key, Kind: WaitApproval, Tool: call.Name, Args: argsOrEmpty(call.Args), AskedAt: s.d.Clock.Now(), Turn: turn, Index: i}
			return true
		}
		e.Steps = append(e.Steps, stepOf(len(e.Steps)+1, call, key, r, s.d.Clock.Now()))
		e.History = append(e.History, toolMessage(call, r))
		// Saved as it completes, so a running execution shows its progress.
		// A failure here is not fatal: finish saves again and reports it.
		_ = s.d.Store.Put(ctx, *e)
	}
	return false
}

func (s *service) call(ctx context.Context, caller Caller, key string, call ToolCall) ToolResult {
	if s.cfg.ToolTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.cfg.ToolTimeout)
		defer cancel()
	}
	r, err := s.d.Gateway.Call(ctx, caller, key, call.Name, call.Args)
	if err != nil {
		return failed(err)
	}
	return r
}

func (s *service) caller(e Execution) Caller {
	return Caller{Tenant: e.Tenant, Execution: e.ID, AgentID: e.AgentID, AgentVersion: e.AgentVersion}
}

// finish stamps an ended execution, releases its sandbox and saves it.
func (s *service) finish(ctx context.Context, e Execution) (Execution, error) {
	if e.Status != Running && e.Status != Waiting {
		e.EndedAt = s.d.Clock.Now()
		if s.d.Sandbox != nil {
			if err := s.d.Sandbox.Release(context.WithoutCancel(ctx), e.ID); err != nil {
				e.Error = strings.TrimSpace(e.Error + "\nrelease sandbox: " + err.Error())
			}
		}
	}
	if err := s.d.Store.Put(ctx, e); err != nil {
		return Execution{}, fmt.Errorf("save execution %s: %w", e.ID, err)
	}
	return e, nil
}
