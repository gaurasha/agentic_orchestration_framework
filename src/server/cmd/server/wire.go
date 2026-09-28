package main

// Adapters from each domain's API to the interfaces other domains need.
// Domains never import each other; these are the only joins.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/agents"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/credentials"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/executions"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/gateway"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/llm"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/sandbox"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/timeline"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/tools"
)

var (
	_ gateway.Tokens      = gatewayTokens{}
	_ gateway.Catalog     = gatewayCatalog{}
	_ gateway.Grants      = gatewayGrants{}
	_ gateway.Credentials = gatewayCredentials{}
	_ gateway.Sandbox     = gatewaySandbox{}
	_ gateway.Upstream    = upstream{}
	_ agents.Registry     = agentsRegistry{}
	_ executions.Agents   = executionsAgents{}
	_ executions.Model    = executionsModel{}
	_ executions.Gateway  = executionsGateway{}
	_ executions.Tokens   = executionsTokens{}
	_ executions.Timeline = executionsTimeline{}
)

// gateway <- credentials: a verified token names its caller.
type gatewayTokens struct{ c credentials.API }

func (a gatewayTokens) Verify(ctx context.Context, token string) (gateway.Caller, error) {
	cl, err := a.c.Verify(ctx, token)
	if errors.Is(err, credentials.ErrUnauthenticated) {
		return gateway.Caller{}, fmt.Errorf("%w: %v", gateway.ErrUnauthenticated, err)
	}
	if err != nil {
		return gateway.Caller{}, err
	}
	return gateway.Caller{Tenant: cl.Tenant, Execution: cl.Execution, AgentID: cl.AgentID, AgentVersion: cl.AgentVersion}, nil
}

// gateway <- tools: the catalog is the registry.
type gatewayCatalog struct{ t tools.API }

func (a gatewayCatalog) Tool(ctx context.Context, tenant, name string) (gateway.Tool, error) {
	t, err := a.t.Get(ctx, tenant, name)
	if errors.Is(err, tools.ErrNotFound) {
		return gateway.Tool{}, fmt.Errorf("%w: %v", gateway.ErrDenied, err)
	}
	if err != nil {
		return gateway.Tool{}, err
	}
	return gateway.Tool{
		Tenant:     t.Tenant,
		Name:       t.Name,
		Kind:       gateway.Kind(t.Kind),
		Spec:       t.Spec,
		Hosts:      t.Hosts,
		Credential: t.Credential,
	}, nil
}

// gateway <- agents: a caller's grant is its pinned version's tools and budget.
type gatewayGrants struct{ a agents.API }

func (a gatewayGrants) For(ctx context.Context, c gateway.Caller) (gateway.Grant, error) {
	v, err := a.a.Get(ctx, c.Tenant, c.AgentID, c.AgentVersion)
	if err != nil {
		return gateway.Grant{}, err
	}
	g := gateway.Grant{Tools: make(map[string]gateway.Allowlist, len(v.Tools)), Budget: gateway.Budget{MaxCalls: v.Budget.MaxCalls}}
	for _, t := range v.Tools {
		g.Tools[t.Name] = gateway.Allowlist(t.Allowlist)
	}
	return g, nil
}

// gateway <- credentials: the real credential, at the moment of the request.
type gatewayCredentials struct{ c credentials.API }

func (a gatewayCredentials) Resolve(ctx context.Context, tenant, ref string) (gateway.Secret, error) {
	return a.c.Resolve(ctx, tenant, ref)
}

// gateway <- sandbox: commands run in the execution's sandbox.
type gatewaySandbox struct{ s sandbox.API }

func (a gatewaySandbox) Exec(ctx context.Context, tenant, execution string, cmd gateway.Command) (gateway.Output, error) {
	out, err := a.s.Exec(ctx, tenant, execution, sandbox.Command{Argv: cmd.Argv, Env: cmd.Env, Timeout: cmd.Timeout})
	if err != nil {
		return gateway.Output{}, err
	}
	return gateway.Output{ExitCode: out.ExitCode, Stdout: out.Stdout, Stderr: out.Stderr}, nil
}

// gateway <- net/http: requests to external services.
type upstream struct{ c *http.Client }

func (a upstream) Do(ctx context.Context, r *http.Request) (*http.Response, error) {
	return a.c.Do(r.WithContext(ctx))
}

// agents <- tools: a definition may list only tools the tenant has.
type agentsRegistry struct{ t tools.API }

func (a agentsRegistry) Has(ctx context.Context, tenant, tool string) (bool, error) {
	_, err := a.t.Get(ctx, tenant, tool)
	if errors.Is(err, tools.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// executions <- agents + tools: an agent version, with each tool described
// as the registry has it.
type executionsAgents struct {
	a agents.API
	t tools.API
}

func (a executionsAgents) Latest(ctx context.Context, tenant, id string) (executions.Agent, error) {
	v, err := a.a.Latest(ctx, tenant, id)
	if err != nil {
		return executions.Agent{}, err
	}
	return a.resolve(ctx, v)
}

func (a executionsAgents) Get(ctx context.Context, tenant, id string, version int) (executions.Agent, error) {
	v, err := a.a.Get(ctx, tenant, id, version)
	if err != nil {
		return executions.Agent{}, err
	}
	return a.resolve(ctx, v)
}

func (a executionsAgents) resolve(ctx context.Context, v agents.Agent) (executions.Agent, error) {
	ag := executions.Agent{ID: v.ID, Version: v.Version, Model: v.Model, Instructions: v.Instructions}
	for _, g := range v.Tools {
		t, err := a.t.Get(ctx, v.Tenant, g.Name)
		if err != nil {
			return executions.Agent{}, fmt.Errorf("agent %s v%d, tool %s: %w", v.ID, v.Version, g.Name, err)
		}
		ag.Tools = append(ag.Tools, executions.Tool{Name: t.Name, Description: t.Description, Params: t.Params, NeedsApproval: g.NeedsApproval})
	}
	return ag, nil
}

// executions <- llm: the loop's prompt becomes a proxy request.
type executionsModel struct{ l llm.API }

func (a executionsModel) Next(ctx context.Context, p executions.Prompt) (executions.Message, error) {
	req := llm.Request{Tenant: p.Tenant, Execution: p.Execution, Model: p.Model, System: p.System}
	for _, m := range p.History {
		req.Messages = append(req.Messages, toLLM(m))
	}
	for _, t := range p.Tools {
		req.Tools = append(req.Tools, llm.ToolSpec{Name: t.Name, Description: t.Description, Params: t.Params})
	}
	res, err := a.l.Complete(ctx, req)
	if err != nil {
		return executions.Message{}, err
	}
	return fromLLM(res.Message), nil
}

func toLLM(m executions.Message) llm.Message {
	out := llm.Message{Role: llm.Role(m.Role), Text: m.Text, ToolCallID: m.ToolCallID, IsError: m.IsError}
	for _, c := range m.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, llm.ToolCall{ID: c.ID, Name: c.Name, Args: c.Args})
	}
	return out
}

func fromLLM(m llm.Message) executions.Message {
	out := executions.Message{Role: string(m.Role), Text: m.Text, ToolCallID: m.ToolCallID, IsError: m.IsError}
	for _, c := range m.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, executions.ToolCall{ID: c.ID, Name: c.Name, Args: c.Args})
	}
	return out
}

// executions <- gateway: every tool call goes through the gateway.
type executionsGateway struct{ g gateway.API }

func (a executionsGateway) Call(ctx context.Context, token, tool string, args json.RawMessage, key string) (executions.ToolResult, error) {
	r, err := a.g.Call(ctx, gateway.Call{Token: token, Tool: tool, Args: args, IdempotencyKey: key})
	if err != nil {
		return executions.ToolResult{}, err
	}
	return executions.ToolResult{Output: r.Output, IsError: r.IsError}, nil
}

// executions <- credentials: the loop's short-lived token.
type executionsTokens struct{ c credentials.API }

func (a executionsTokens) Issue(ctx context.Context, tenant, execution, agentID string, agentVersion int, ttl time.Duration) (string, error) {
	return a.c.Issue(ctx, credentials.Claims{Tenant: tenant, Execution: execution, AgentID: agentID, AgentVersion: agentVersion}, ttl)
}

// executions <- timeline: each step is recorded.
type executionsTimeline struct{ t timeline.API }

func (a executionsTimeline) Record(ctx context.Context, e executions.Event) error {
	_, err := a.t.Append(ctx, timeline.Event{Tenant: e.Tenant, Execution: e.Execution, Kind: timeline.Kind(e.Kind), Data: e.Data})
	return err
}
