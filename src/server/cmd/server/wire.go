package main

// newApp builds every domain on its in-memory adapters and joins them.
// Domains never import each other; the adapters below are the only joins.

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/agents"
	agentsmem "github.com/gaurasha/agentic_orchestration_framework/src/server/internal/agents/deps/memory"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/credentials"
	credsmem "github.com/gaurasha/agentic_orchestration_framework/src/server/internal/credentials/deps/memory"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/egress"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/executions"
	execsmem "github.com/gaurasha/agentic_orchestration_framework/src/server/internal/executions/deps/memory"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/executions/deps/mock"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/gateway"
	gwmem "github.com/gaurasha/agentic_orchestration_framework/src/server/internal/gateway/deps/memory"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/sandbox"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/sandbox/deps/docker"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/sandbox/deps/local"
	sandboxmem "github.com/gaurasha/agentic_orchestration_framework/src/server/internal/sandbox/deps/memory"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/tools"
	toolsmem "github.com/gaurasha/agentic_orchestration_framework/src/server/internal/tools/deps/memory"
)

type clock interface{ Now() time.Time }

type appConfig struct {
	SigningKey      string
	EgressURL       string // what a sandboxed command gets as EGRESS and as its HTTP proxy
	CAFile          string // where the platform CA's certificate is written for sandboxes to trust; "" writes none
	Sandbox         string // "local" or "docker"
	SandboxImage    string
	AllowHTTPEgress bool           // for local stand-ins of external services
	UpstreamRoots   *x509.CertPool // roots egress trusts for external services; nil means the system's
}

// caLife is long: the key dies with the process, so expiry adds nothing.
const caLife = 10 * 365 * 24 * time.Hour

type app struct {
	echoCount   atomic.Int64 // requests /demo/echo has served
	gitCount    atomic.Int64 // ls-remotes /demo/git has served with the real credential
	agents      agents.API
	tools       tools.API
	credentials credentials.API
	executions  executions.API
	sandbox     sandbox.API
	egress      egress.API
	ca          *egress.CA // signs a certificate per host for the CONNECT form of egress
	egressHost  string     // how a sandboxed command addresses egress; a proxy request to it is served here
	log         *slog.Logger
}

func newApp(cfg appConfig, log *slog.Logger, clk clock) (*app, error) {
	ids := randomIDs{}
	ca, err := egress.NewCA(rand.Reader, clk.Now(), caLife)
	if err != nil {
		return nil, err
	}
	if cfg.CAFile != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.CAFile), 0o755); err != nil {
			return nil, fmt.Errorf("ca file: %w", err)
		}
		if err := os.WriteFile(cfg.CAFile, ca.PEM(), 0o644); err != nil {
			return nil, fmt.Errorf("ca file: %w", err)
		}
	}
	egressURL, err := url.Parse(cfg.EgressURL)
	if err != nil {
		return nil, fmt.Errorf("egress url: %w", err)
	}
	creds := credentials.New(credentials.Deps{
		Secrets: credsmem.NewSecrets(), Placeholders: credsmem.NewPlaceholders(),
		Key: credentials.NewSecret(cfg.SigningKey), Clock: clk, Random: ids,
	})
	tl := tools.New(tools.Deps{Store: toolsmem.NewStore()})
	ag := agents.New(agents.Deps{Store: agentsmem.NewStore(), Registry: agentsRegistry{tl}, Clock: clk, IDs: ids})
	var runtime sandbox.Runtime = local.Runtime{CAFile: cfg.CAFile}
	if cfg.Sandbox == "docker" {
		runtime = docker.Runtime{Image: cfg.SandboxImage, CAFile: cfg.CAFile}
	}
	sb := sandbox.New(sandbox.Deps{Runtime: runtime, Store: sandboxmem.NewStore()})
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{RootCAs: cfg.UpstreamRoots, MinVersion: tls.VersionTLS12}
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: noRedirect, Transport: transport}
	gw := gateway.New(gateway.Deps{
		Catalog:      gatewayCatalog{tl},
		Grants:       gatewayGrants{ag},
		Budgets:      gwmem.NewBudgets(),
		Journal:      gwmem.NewJournal(),
		Approvals:    gwmem.NewApprovals(),
		Credentials:  gatewayCredentials{creds},
		Placeholders: gatewayPlaceholders{creds, clk},
		Sandbox:      gatewaySandbox{sb},
		Upstream:     upstream{client},
		Log:          log,
	}, gateway.Config{MaxOutput: 4096, Egress: cfg.EgressURL, ExecTimeout: 60 * time.Second, PlaceholderTTL: 5 * time.Minute})
	eg := egress.New(egress.Deps{Placeholders: egressPlaceholders{creds}, Upstream: upstream{client}, Log: log},
		egress.Config{AllowHTTP: cfg.AllowHTTPEgress})
	ex := executions.New(executions.Deps{
		Agents: executionsAgents{ag, tl}, Model: mock.Model{}, Gateway: executionsGateway{gw}, Sandbox: sb,
		Store: execsmem.NewStore(), Clock: clk, IDs: ids,
	}, executions.Config{MaxSteps: 10, ToolTimeout: 90 * time.Second})
	return &app{
		agents: ag, tools: tl, credentials: creds, executions: ex, sandbox: sb, egress: eg,
		ca: ca, egressHost: egressURL.Host, log: log,
	}, nil
}

var (
	_ gateway.Catalog      = gatewayCatalog{}
	_ gateway.Grants       = gatewayGrants{}
	_ gateway.Credentials  = gatewayCredentials{}
	_ gateway.Placeholders = gatewayPlaceholders{}
	_ gateway.Sandbox      = gatewaySandbox{}
	_ gateway.Upstream     = upstream{}
	_ egress.Placeholders  = egressPlaceholders{}
	_ agents.Registry      = agentsRegistry{}
	_ executions.Agents    = executionsAgents{}
	_ executions.Gateway   = executionsGateway{}
	_ tenantTokens         = tenantTokensOf{}
)

// randomIDs makes IDs and placeholder tokens from crypto/rand.
type randomIDs struct{}

func (randomIDs) NewID() string { return random(8) }
func (randomIDs) Token() string { return random(16) }

func random(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// noRedirect keeps the credential on the host the tool or placeholder names.
func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// auth <- credentials: the UI's tenant token.
type tenantTokensOf struct{ c credentials.API }

func (a tenantTokensOf) Mint(ctx context.Context, tenant string, ttl time.Duration) (string, error) {
	return a.c.Issue(ctx, tenant, ttl)
}

func (a tenantTokensOf) Verify(ctx context.Context, token string) (string, error) {
	cl, err := a.c.Verify(ctx, token)
	return cl.Tenant, err
}

// gateway <- tools: the catalog is the registry.
type gatewayCatalog struct{ t tools.API }

func (a gatewayCatalog) Tool(ctx context.Context, tenant, name string) (gateway.Tool, error) {
	t, err := a.t.Get(ctx, tenant, name)
	if errors.Is(err, tools.ErrNotFound) {
		return gateway.Tool{}, fmt.Errorf("%w: tool %s is not in the registry", gateway.ErrDenied, name)
	}
	if err != nil {
		return gateway.Tool{}, err
	}
	out := gateway.Tool{Name: t.Name, Kind: gateway.Kind(t.Kind), Params: t.Params}
	if t.HTTP != nil {
		out.Method, out.URL = t.HTTP.Method, t.HTTP.URL
	}
	if t.Exec != nil {
		out.Argv, out.Timeout = t.Exec.Argv, time.Duration(t.Exec.TimeoutSeconds)*time.Second
	}
	if c := t.Credential; c != nil {
		out.Credential = &gateway.Credential{Ref: c.Ref, Header: c.Header, Prefix: c.Prefix, Env: c.Env, Hosts: c.Hosts}
	}
	return out, nil
}

// gateway <- agents: a caller's grant is its pinned version's tools and budget.
type gatewayGrants struct{ a agents.API }

func (a gatewayGrants) For(ctx context.Context, c gateway.Caller) (gateway.Grant, error) {
	v, err := a.a.Get(ctx, c.Tenant, c.AgentID, c.AgentVersion)
	if err != nil {
		return gateway.Grant{}, err
	}
	g := gateway.Grant{Tools: make(map[string]gateway.ToolGrant, len(v.Tools)), Budget: gateway.Budget{MaxCalls: v.Budget.MaxCalls}}
	for _, t := range v.Tools {
		g.Tools[t.Name] = gateway.ToolGrant{Allowlist: t.Allowlist, NeedsApproval: t.NeedsApproval}
	}
	return g, nil
}

// gateway <- credentials: the real credential, at the moment of the request.
type gatewayCredentials struct{ c credentials.API }

func (a gatewayCredentials) Resolve(ctx context.Context, tenant, ref string) (gateway.Secret, error) {
	return a.c.Resolve(ctx, tenant, ref)
}

// gateway <- credentials: a placeholder per exec call.
type gatewayPlaceholders struct {
	c   credentials.API
	clk clock
}

func (a gatewayPlaceholders) Issue(ctx context.Context, p gateway.PlaceholderRequest) (string, error) {
	return a.c.IssuePlaceholder(ctx, credentials.Placeholder{
		Tenant: p.Tenant, Execution: p.Execution, Ref: p.Ref, Hosts: p.Hosts, Expires: a.clk.Now().Add(p.TTL),
	})
}

func (a gatewayPlaceholders) Retire(ctx context.Context, value string) error {
	return a.c.RetirePlaceholder(ctx, value)
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

// egress <- credentials: a live placeholder for the real credential.
type egressPlaceholders struct{ c credentials.API }

func (a egressPlaceholders) Resolve(ctx context.Context, value, host string) (egress.Secret, error) {
	sec, err := a.c.ResolvePlaceholder(ctx, value, host)
	switch {
	case errors.Is(err, credentials.ErrUnauthenticated):
		return nil, rewrap(err, credentials.ErrUnauthenticated, egress.ErrUnauthenticated)
	case errors.Is(err, credentials.ErrDenied):
		return nil, rewrap(err, credentials.ErrDenied, egress.ErrDenied)
	case err != nil:
		return nil, err
	}
	return sec, nil
}

// rewrap moves an error from one domain's sentinel to another's, keeping
// the reason and not repeating the sentinel's text.
func rewrap(err, from, to error) error {
	return fmt.Errorf("%w: %s", to, strings.TrimPrefix(err.Error(), from.Error()+": "))
}

// gateway, egress <- net/http: requests to external services.
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

// executions <- agents + tools: an agent version, each tool described as
// the registry has it.
type executionsAgents struct {
	a agents.API
	t tools.API
}

func (a executionsAgents) Latest(ctx context.Context, tenant, id string) (executions.Agent, error) {
	v, err := a.a.Latest(ctx, tenant, id)
	if err != nil {
		return executions.Agent{}, err
	}
	return a.resolve(ctx, tenant, v)
}

func (a executionsAgents) Get(ctx context.Context, tenant, id string, version int) (executions.Agent, error) {
	v, err := a.a.Get(ctx, tenant, id, version)
	if err != nil {
		return executions.Agent{}, err
	}
	return a.resolve(ctx, tenant, v)
}

func (a executionsAgents) resolve(ctx context.Context, tenant string, v agents.Agent) (executions.Agent, error) {
	ag := executions.Agent{ID: v.ID, Version: v.Version, Model: v.Model, Instructions: v.Instructions}
	for _, g := range v.Tools {
		t, err := a.t.Get(ctx, tenant, g.Name)
		if err != nil {
			return executions.Agent{}, fmt.Errorf("agent %s v%d, tool %s: %w", v.ID, v.Version, g.Name, err)
		}
		ag.Tools = append(ag.Tools, executions.Tool{Name: t.Name, Description: t.Description, Params: t.Params})
	}
	return ag, nil
}

// executions <- gateway: every tool call goes through the gateway. A
// refusal, or a call that needs approval, becomes a result, so the loop
// reads the reason or pauses.
type executionsGateway struct{ g gateway.API }

func gatewayCaller(c executions.Caller) gateway.Caller {
	return gateway.Caller{Tenant: c.Tenant, Execution: c.Execution, AgentID: c.AgentID, AgentVersion: c.AgentVersion}
}

func (a executionsGateway) Approve(ctx context.Context, c executions.Caller, key string) error {
	return a.g.Approve(ctx, gatewayCaller(c), key)
}

func (a executionsGateway) Call(ctx context.Context, c executions.Caller, key, tool string, args json.RawMessage) (executions.ToolResult, error) {
	r, err := a.g.Call(ctx, gateway.Call{Caller: gatewayCaller(c), Tool: tool, Args: args, Key: key})
	switch {
	case errors.Is(err, gateway.ErrApprovalNeeded):
		return executions.ToolResult{Outcome: executions.NeedsApproval, Output: err.Error()}, nil
	case errors.Is(err, gateway.ErrDenied), errors.Is(err, gateway.ErrExhausted), errors.Is(err, gateway.ErrConflict):
		return executions.ToolResult{Outcome: executions.Refused, Output: err.Error()}, nil
	case err != nil:
		return executions.ToolResult{}, err
	}
	out := executions.ToolResult{Outcome: executions.OK, Kind: string(r.Kind), Replayed: r.Replayed}
	if r.IsError {
		out.Outcome = executions.Errored
	}
	switch r.Kind {
	case gateway.KindExec:
		code := r.ExitCode
		out.ExitCode = &code
		out.Output = r.Output
	default:
		out.Output = fmt.Sprintf("HTTP %d\n%s", r.Status, r.Output)
	}
	return out, nil
}
