package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type service struct {
	d   Deps
	cfg Config
}

// Call checks in this order: tool, grant, schema, allowlist, the journal,
// approval, budget. Then it runs the tool and hides the credential from
// the result. The log names the call and each decision, before and after
// the run, never the credential or a placeholder.
func (s *service) Call(ctx context.Context, c Call) (Result, error) {
	log := s.d.Log.With("tenant", c.Caller.Tenant, "execution", c.Caller.Execution, "tool", c.Tool, "key", c.Key)
	res, err := s.call(ctx, c, log)
	switch {
	case errors.Is(err, ErrApprovalNeeded):
		log.Info("tool call needs approval")
	case errors.Is(err, ErrDenied), errors.Is(err, ErrExhausted), errors.Is(err, ErrConflict):
		log.Info("tool call refused", "reason", err)
	case err != nil:
		log.Error("tool call failed", "err", err)
	case res.Replayed:
		log.Info("tool call replayed from the journal")
	case res.Kind == KindExec:
		log.Info("tool call finished", "kind", res.Kind, "exit", res.ExitCode)
	default:
		log.Info("tool call finished", "kind", res.Kind, "status", res.Status)
	}
	return res, err
}

func (s *service) Approve(ctx context.Context, c Caller, key string) error {
	if key == "" {
		return fmt.Errorf("%w: an approval needs a call key", ErrDenied)
	}
	s.d.Log.Info("tool call approved", "tenant", c.Tenant, "execution", c.Execution, "key", key)
	return s.d.Approvals.Put(ctx, c.Tenant, key)
}

func (s *service) call(ctx context.Context, c Call, log logger) (Result, error) {
	tool, err := s.d.Catalog.Tool(ctx, c.Caller.Tenant, c.Tool)
	if err != nil {
		return Result{}, err
	}
	grant, err := s.d.Grants.For(ctx, c.Caller)
	if err != nil {
		return Result{}, fmt.Errorf("load grant: %w", err)
	}
	if err := checkSchema(tool.Params, c.Args); err != nil {
		return Result{}, err
	}
	args, err := argsOf(c.Args)
	if err != nil {
		return Result{}, err
	}
	tg, err := checkGrant(grant, c.Tool, args)
	if err != nil {
		return Result{}, err
	}

	// The journal first: a retry of a finished call must not run twice, or
	// ask for approval or budget again.
	if c.Key != "" {
		saved, done, err := s.d.Journal.Begin(ctx, c.Caller.Tenant, c.Key)
		if err != nil {
			return Result{}, err
		}
		if done {
			saved.Replayed = true
			return saved, nil
		}
	}
	res, err := s.run(ctx, c, tool, tg, grant, args, log)
	if errors.Is(err, ErrUncertain) {
		// The call left; what it did is unknown. That is a result, saved
		// under the key, so a retry replays it instead of sending again.
		res, err = Result{Kind: tool.Kind, IsError: true, Uncertain: true,
			Output: fmt.Sprintf("error: %v; the call was sent and its effect is unknown, so it is not sent again: a retry replays this result", err)}, nil
	}
	if c.Key != "" {
		if err != nil { // refused, or never sent: the key is free for a retry
			if aerr := s.d.Journal.Abort(context.WithoutCancel(ctx), c.Caller.Tenant, c.Key); aerr != nil {
				log.Error("release journal claim", "err", aerr)
			}
		} else if ferr := s.d.Journal.Finish(context.WithoutCancel(ctx), c.Caller.Tenant, c.Key, res); ferr != nil {
			return Result{}, fmt.Errorf("save result: %w", ferr)
		}
	}
	return res, err
}

// run holds the checks that happen once per call, then the call itself.
func (s *service) run(ctx context.Context, c Call, tool Tool, tg ToolGrant, grant Grant, args map[string]string, log logger) (Result, error) {
	if tg.NeedsApproval {
		ok := false
		if c.Key != "" {
			var err error
			if ok, err = s.d.Approvals.Has(ctx, c.Caller.Tenant, c.Key); err != nil {
				return Result{}, fmt.Errorf("check approval: %w", err)
			}
		}
		if !ok {
			return Result{}, fmt.Errorf("%w: tool %s needs a human's approval", ErrApprovalNeeded, c.Tool)
		}
	}
	var env map[string]string
	var url string
	switch tool.Kind {
	case KindHTTP:
		u, err := buildURL(tool.URL, args)
		if err != nil {
			return Result{}, err
		}
		url = u.String()
	case KindExec:
		var err error
		if env, err = commandEnv(args, s.cfg.Egress, "", ""); err != nil {
			return Result{}, err
		}
	default:
		return Result{}, fmt.Errorf("%w: tool %s has kind %q", ErrDenied, tool.Name, tool.Kind)
	}
	if err := s.d.Budgets.Spend(ctx, c.Caller.Execution, grant.Budget); err != nil {
		return Result{}, err
	}
	log.Info("tool call started", "kind", tool.Kind)
	if tool.Kind == KindExec {
		return s.callExec(ctx, c.Caller, tool, env)
	}
	return s.callHTTP(ctx, c.Caller, tool, url)
}

// callHTTP makes the request with the real credential in its header.
func (s *service) callHTTP(ctx context.Context, caller Caller, tool Tool, url string) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, tool.Method, url, nil)
	if err != nil {
		return Result{}, fmt.Errorf("build request: %w", err)
	}
	var secret string
	if cr := tool.Credential; cr != nil {
		sec, err := s.d.Credentials.Resolve(ctx, caller.Tenant, cr.Ref)
		if err != nil {
			return Result{}, fmt.Errorf("credential %s: %w", cr.Ref, err)
		}
		secret = sec.Reveal()
		req.Header.Set(cr.Header, cr.Prefix+secret)
	}
	// From here the request may have reached the service, so a failure is
	// uncertain, never a reason to send again.
	resp, err := s.d.Upstream.Do(ctx, req)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %s %s: %v", ErrUncertain, tool.Method, req.URL.Host, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(s.cfg.MaxOutput)+1))
	if err != nil {
		return Result{}, fmt.Errorf("%w: %s %s: reading the response: %v", ErrUncertain, tool.Method, req.URL.Host, err)
	}
	return Result{
		Kind:    KindHTTP,
		Status:  resp.StatusCode,
		Output:  redact(string(body), secret, s.cfg.MaxOutput),
		IsError: resp.StatusCode < 200 || resp.StatusCode > 299,
	}, nil
}

// callExec runs the command in the sandbox with a placeholder where the
// credential goes; the placeholder is retired as soon as the command ends.
// The real value is resolved here only to hide it from the output, in
// case the sandbox somehow saw it.
func (s *service) callExec(ctx context.Context, caller Caller, tool Tool, env map[string]string) (Result, error) {
	var secret string
	if cr := tool.Credential; cr != nil {
		sec, err := s.d.Credentials.Resolve(ctx, caller.Tenant, cr.Ref)
		if err != nil {
			return Result{}, fmt.Errorf("credential %s: %w", cr.Ref, err)
		}
		secret = sec.Reveal()
		ph, err := s.d.Placeholders.Issue(ctx, PlaceholderRequest{
			Tenant: caller.Tenant, Execution: caller.Execution, Ref: cr.Ref, Hosts: cr.Hosts, TTL: s.cfg.PlaceholderTTL,
		})
		if err != nil {
			return Result{}, fmt.Errorf("placeholder for %s: %w", cr.Ref, err)
		}
		defer func() {
			if err := s.d.Placeholders.Retire(context.WithoutCancel(ctx), ph); err != nil {
				s.d.Log.Error("retire placeholder", "err", err)
			}
		}()
		env[cr.Env] = ph
	}
	timeout := tool.Timeout
	if timeout <= 0 {
		timeout = s.cfg.ExecTimeout
	}
	out, err := s.d.Sandbox.Exec(ctx, caller.Tenant, caller.Execution, Command{Argv: tool.Argv, Env: env, Timeout: timeout})
	if err != nil { // the command may have run, in part or in full
		return Result{}, fmt.Errorf("%w: %v", ErrUncertain, err)
	}
	return Result{
		Kind:     KindExec,
		ExitCode: out.ExitCode,
		Output:   redact(commandOutput(out), secret, s.cfg.MaxOutput),
		IsError:  out.ExitCode != 0,
	}, nil
}

// logger is what the service needs of slog, so a call's log lines share
// their attributes.
type logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}
