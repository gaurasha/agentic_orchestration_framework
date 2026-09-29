package main

// seed loads the README demos' data for one tenant at start: everything is
// in memory, so without it each restart means setting the demos up again
// by hand. It calls the same domain APIs as the /v1 handlers, so seeded
// data is exactly what the UI would have made.

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/agents"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/credentials"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/executions"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/tools"
)

// seedOptions says what to load.
type seedOptions struct {
	Tenant      string
	APIHost     string // the runs call /demo/echo here; it stands in for the external service
	RunCommands bool   // false for a sandbox that may not reach egress, such as Docker on Colima
	GitHubToken string // stored as github_token, and a `github` agent granted the tools that use it; "" for neither
	RunGitHub   bool   // also run the github agent once, in the background, so the demo run is there to look at
}

// seed needs the API and egress listening. It never needs a GitHub token
// or the network to succeed: the run against GitHub, if asked for, starts
// after seed returns and reports in the log.
func (a *app) seed(ctx context.Context, o seedOptions) error {
	tenant, apiHost, runCommands, githubToken := o.Tenant, o.APIHost, o.RunCommands, o.GitHubToken
	if err := a.credentials.Store(ctx, tenant, "demo_key", credentials.NewSecret("demo-s3cret-value")); err != nil {
		return fmt.Errorf("seed credential: %w", err)
	}
	if githubToken != "" {
		if err := a.credentials.Store(ctx, tenant, "github_token", credentials.NewSecret(githubToken)); err != nil {
			return fmt.Errorf("seed credential github_token: %w", err)
		}
	}

	key := &tools.Credential{Ref: "demo_key", Env: "DEMO_KEY", Hosts: []string{apiHost}}
	for _, t := range []tools.Tool{
		{
			Name: "echo", Description: "Looks up a city on the demo echo service", Kind: tools.KindHTTP,
			HTTP:       &tools.HTTP{Method: "GET", URL: "http://" + apiHost + "/demo/echo?city={city}"},
			Params:     json.RawMessage(`{"type":"object","required":["city"],"properties":{"city":{"type":"string"}}}`),
			Credential: &tools.Credential{Ref: "demo_key", Header: "Authorization", Prefix: "Bearer "},
		},
		{
			Name: "deploy", Description: "Deploys to an environment", Kind: tools.KindHTTP,
			HTTP:   &tools.HTTP{Method: "GET", URL: "http://" + apiHost + "/demo/echo?env={env}"},
			Params: json.RawMessage(`{"type":"object","required":["env"],"properties":{"env":{"type":"string","enum":["staging","prod"]}}}`),
		},
		{
			Name: "whoami", Description: "Calls the echo service from the sandbox, through egress", Kind: tools.KindExec,
			Exec: &tools.Exec{Argv: []string{"sh", "-c",
				`echo "env: $DEMO_KEY"; curl -sS -H "Authorization: Bearer $DEMO_KEY" "$EGRESS/http/` + apiHost + `/demo/echo?who=$ARG_who"`}},
			Credential: key,
		},
		{
			Name: "elsewhere", Description: "Tries a host the credential is not bound to; egress answers 403", Kind: tools.KindExec,
			Exec: &tools.Exec{Argv: []string{"sh", "-c",
				`curl -sS -o /dev/null -w "egress answered %{http_code}" -H "Authorization: Bearer $DEMO_KEY" "$EGRESS/http/other.example/x"`}},
			Credential: key,
		},
		{
			// Needs a credential github_token, stored in the UI. git speaks
			// plain HTTP to egress with the placeholder as its password;
			// egress makes the HTTPS call to github.com with the real token.
			Name: "git_refs", Description: "Lists a GitHub repository's branches with git, through egress", Kind: tools.KindExec,
			Exec: &tools.Exec{Argv: []string{"sh", "-c",
				`echo "env: $GH_TOKEN"; git -c credential.helper= ls-remote --heads "http://x-access-token:$GH_TOKEN@${EGRESS#http://}/https/github.com/$ARG_repo.git"`}},
			Params:     json.RawMessage(`{"type":"object","required":["repo"],"properties":{"repo":{"type":"string","description":"owner/name"}}}`),
			Credential: &tools.Credential{Ref: "github_token", Env: "GH_TOKEN", Hosts: []string{"github.com"}},
		},
		{
			// gh fixes its host, so it cannot be pointed at egress's path form.
			// Instead it takes HTTPS_PROXY from its environment and sends a
			// CONNECT to egress, which answers with the platform CA's certificate
			// for api.github.com (the sandbox trusts the CA) and swaps the
			// placeholder inside the TLS connection. Needs github_token.
			Name: "gh_prs", Description: "Lists a GitHub repository's open pull requests with gh, through egress", Kind: tools.KindExec,
			Exec: &tools.Exec{Argv: []string{"sh", "-c",
				`echo "env: $GH_TOKEN"; export GH_NO_UPDATE_NOTIFIER=1; echo "login: $(gh api user --jq .login)"; gh pr list --repo "$ARG_repo" --limit 3`}},
			Params:     json.RawMessage(`{"type":"object","required":["repo"],"properties":{"repo":{"type":"string","description":"owner/name"}}}`),
			Credential: &tools.Credential{Ref: "github_token", Env: "GH_TOKEN", Hosts: []string{"api.github.com"}},
		},
		{
			// The same placeholder sent straight to GitHub, around egress:
			// GitHub does not know it.
			Name: "gh_direct", Description: "Sends the placeholder to api.github.com directly, not through egress", Kind: tools.KindExec,
			// Short timeouts: this is the one call that needs the sandbox's own
			// route to the internet, which a Docker VM may lack.
			Exec: &tools.Exec{Argv: []string{"sh", "-c",
				`curl -sS -m 10 --noproxy '*' -H "Authorization: token $GH_TOKEN" https://api.github.com/user`}, TimeoutSeconds: 15},
			Credential: &tools.Credential{Ref: "github_token", Env: "GH_TOKEN", Hosts: []string{"api.github.com"}},
		},
		{
			// gh pointed at another host: egress refuses before anything leaves.
			Name: "gh_elsewhere", Description: "Points gh at a host the token is not bound to; egress answers 403", Kind: tools.KindExec,
			Exec: &tools.Exec{Argv: []string{"sh", "-c",
				`GH_NO_UPDATE_NOTIFIER=1 GH_HOST=api.example.com GH_ENTERPRISE_TOKEN=$GH_TOKEN gh api user 2>&1`}},
			Credential: &tools.Credential{Ref: "github_token", Env: "GH_TOKEN", Hosts: []string{"api.github.com"}},
		},
		{
			// The same as git_refs against the fake git server, which lists
			// its branch only to the real stored value. No GitHub token needed.
			Name: "git_refs_demo", Description: "Lists the demo git server's branches with git, through egress", Kind: tools.KindExec,
			Exec: &tools.Exec{Argv: []string{"sh", "-c",
				`echo "env: $DEMO_KEY"; git -c credential.helper= ls-remote --heads "http://x-access-token:$DEMO_KEY@${EGRESS#http://}/http/` + apiHost + `/demo/git/` + tenant + `/demo_key/$ARG_repo.git"`}},
			Params:     json.RawMessage(`{"type":"object","required":["repo"],"properties":{"repo":{"type":"string","description":"owner/name"}}}`),
			Credential: key,
		},
		{
			Name: "write", Description: "Writes text to note.txt in the run's files", Kind: tools.KindExec,
			Exec:   &tools.Exec{Argv: []string{"sh", "-c", `echo "$ARG_text" > note.txt`}},
			Params: json.RawMessage(`{"type":"object","required":["text"],"properties":{"text":{"type":"string"}}}`),
		},
		{
			Name: "read", Description: "Prints note.txt from the run's files", Kind: tools.KindExec,
			Exec: &tools.Exec{Argv: []string{"cat", "note.txt"}},
		},
	} {
		t.Tenant = tenant
		if _, err := a.tools.Put(ctx, t); err != nil {
			return fmt.Errorf("seed tool %s: %w", t.Name, err)
		}
	}

	// Ten agents, one per part of the design, and a run of each so the
	// Executions tab shows every part at once. Each run's description says
	// what to expect and how to read its steps.
	echoOnly := func(name, instructions string, maxCalls int, grant agents.ToolGrant) agents.Definition {
		return agents.Definition{Name: name, Model: "mock", Instructions: instructions, Budget: agents.Budget{MaxCalls: maxCalls}, Tools: []agents.ToolGrant{grant}}
	}
	create := func(def agents.Definition) (agents.Agent, error) {
		ag, err := a.agents.Create(ctx, tenant, def)
		if err != nil {
			return agents.Agent{}, fmt.Errorf("seed agent %s: %w", def.Name, err)
		}
		return ag, nil
	}
	run := func(r executions.Run) (executions.Execution, error) {
		e, err := a.executions.Start(ctx, tenant, r)
		if err != nil {
			return executions.Execution{}, fmt.Errorf("seed execution for %s: %w", r.AgentID, err)
		}
		return e, nil
	}
	echo := agents.ToolGrant{Name: "echo"}
	london := agents.ToolGrant{Name: "echo", Allowlist: map[string][]string{"city": {"London"}}}

	// 1. The gateway adds the credential and hides it.
	ag, err := create(echoOnly("01-credential-hidden", "Look up London.", 0, echo))
	if err != nil {
		return err
	}
	if _, err := run(executions.Run{AgentID: ag.ID, Input: "echo {\"city\":\"London\"}", Description: `Demo 1 of 10: a credential the agent never holds. The tool echo carries the stored credential demo_key as "Authorization: Bearer …"; the gateway adds it to the request and hides it in the response.
Expect: one step, ok. The echo service (a fake external API built into the server) reports the header it received as "Authorization: Bearer [secret]": the real value reached the service, and the model, this page and the API only ever see [secret]. No API response or log line contains the value; a test fails if one does.`}); err != nil {
		return err
	}

	// 2. What the gateway refuses before anything runs.
	ag, err = create(echoOnly("02-refusals", "Look up cities.", 0, london))
	if err != nil {
		return err
	}
	if _, err := run(executions.Run{AgentID: ag.ID, Input: "echo {\"city\":\"Tokyo\"}\ndelete_repo {}\necho {\"city\":5}\necho {}\necho {\"city\":\"London\"}", Description: `Demo 2 of 10: the checks that run before a call, in a fixed order: the tool must be in the registry, the agent version must grant it, the arguments must match the tool's JSON Schema, then the grant's allowlist. This agent is granted echo with the allowlist city: London.
Expect: step 1 refused, Tokyo is not in the allowlist. Step 2 refused, delete_repo is not in the registry. Step 3 refused by the schema, city must be a string. Step 4 refused by the schema, city is required. Step 5 ok.
Each refusal names its reason, and the model reads it and carries on; a refused call never reaches the service and costs no budget.`}); err != nil {
		return err
	}

	// 3. The budget.
	ag, err = create(echoOnly("03-budget", "Look up London, twice at most.", 2, echo))
	if err != nil {
		return err
	}
	if _, err := run(executions.Run{AgentID: ag.ID, Input: "delete_repo {}\necho {\"city\":\"London\"}\necho {\"city\":\"London\"}\necho {\"city\":\"London\"}", Description: `Demo 3 of 10: a budget of 2 tool calls per run, enforced by the gateway.
Expect: step 1 refused (not in the registry) and it costs nothing. Steps 2 and 3 ok: the two calls. Step 4 refused, "budget exhausted: 2 of 2 calls used". The budget is checked after every other check, so refusals and replays never spend it.`}); err != nil {
		return err
	}

	// 4. The journal: a retry is served from it.
	ag, err = create(echoOnly("04-retry-journal", "Look up London.", 0, echo))
	if err != nil {
		return err
	}
	e, err := run(executions.Run{AgentID: ag.ID, Input: "echo {\"city\":\"London\"}", Description: `Demo 4 of 10: idempotent retries. Every call carries a key, execution:turn:index, and the gateway journals its result under that key.
Expect: step 1 ok, with a "request" number from the echo service. Step 2, marked "replayed: nothing ran", was made by clicking Retry on step 1: the gateway returned the journaled result, and the same "request" number shows the service saw one request, not two. Click Retry again: the same.`})
	if err != nil {
		return err
	}
	if _, err := a.executions.Retry(ctx, tenant, e.ID, 1); err != nil {
		return fmt.Errorf("seed retry: %w", err)
	}

	// 5. Approval.
	approver := agents.Definition{
		Name: "05-approval", Model: "mock", Instructions: "Deploy when asked, then check the weather.",
		Budget: agents.Budget{MaxCalls: 5},
		Tools:  []agents.ToolGrant{{Name: "deploy", NeedsApproval: true}, echo},
	}
	ag, err = create(approver)
	if err != nil {
		return err
	}
	if _, err := run(executions.Run{AgentID: ag.ID, Input: "deploy {\"env\":\"prod\"}\necho {\"city\":\"London\"}", Description: `Demo 5 of 10: a human in the loop. deploy is granted with "a human approves each call", so the gateway refuses it until an approval for this one call is on file, and the run waits.
Expect: the run is waiting for your approval with no steps run. Approve: the deploy runs, marked "approved", then the echo runs and the run finishes. Or Reject with a reason: the reason becomes the step's outcome, the model reads it, and the run goes on to the echo.
An approval covers one call; another call to deploy would wait again.`}); err != nil {
		return err
	}

	// 6. A question, and cancel.
	approver.Name, approver.Instructions = "06-question", "Ask which environment, then deploy."
	ag, err = create(approver)
	if err != nil {
		return err
	}
	if _, err := run(executions.Run{AgentID: ag.ID, Input: "ask Which environment?\ndeploy {\"env\":\"staging\"}", Description: `Demo 6 of 10: the model asks the user something, and a waiting run can be cancelled.
Expect: the run is waiting at the question "Which environment?". Answer it: the answer goes to the model as a message, and the run goes on to deploy, which needs approval, so it waits again, this time for your decision. An answer never counts as an approval, and a decision never counts as an answer.
Or Cancel the run with a reason: it ends as cancelled and the deploy never runs (the echo service's request counter does not move).`}); err != nil {
		return err
	}

	// 7. Loop detection.
	ag, err = create(echoOnly("07-loop-detection", "Look up London.", 10, echo))
	if err != nil {
		return err
	}
	if _, err := run(executions.Run{AgentID: ag.ID, Input: strings.Repeat("echo {\"city\":\"London\"}\n", 5), Description: `Demo 7 of 10: a looping agent is stopped. The same call, tool and arguments, is refused once it has already been made 3 times in the run.
Expect: steps 1 to 3 ok. Steps 4 and 5 refused, "you have already made this exact call 3 times; it was not run again". The model is told, so it can change course; the budget of 10 was not burned. A step cap ends a run that never gives a final answer.`}); err != nil {
		return err
	}

	// 8. Version pinning: the agent is edited after its run.
	pinned := echoOnly("08-version-pinning", "Look up London.", 0, echo)
	ag, err = create(pinned)
	if err != nil {
		return err
	}
	if _, err := run(executions.Run{AgentID: ag.ID, Input: "echo {\"city\":\"London\"}", Description: `Demo 8 of 10: an execution stays on the agent version it started with. This run started on v1; the agent was then edited to v2 (see the Agents tab: its instructions changed), and saved once more unchanged, which made no new version.
Expect: this run says "ran on v1". Run the agent again from the form: that run says "ran on v2". Editing an agent never changes what an earlier run was allowed to do.`}); err != nil {
		return err
	}
	pinned.Instructions = "Look up London only. Be brief."
	if _, err := a.agents.Update(ctx, tenant, ag.ID, pinned); err != nil {
		return fmt.Errorf("seed agent %s v2: %w", pinned.Name, err)
	}
	if _, err := a.agents.Update(ctx, tenant, ag.ID, pinned); err != nil { // unchanged: still v2
		return fmt.Errorf("seed agent %s unchanged save: %w", pinned.Name, err)
	}

	// 9. The sandbox and egress.
	ag, err = create(agents.Definition{
		Name: "09-sandbox-egress", Model: "mock", Instructions: "Run commands in the sandbox.",
		Budget: agents.Budget{MaxCalls: 10},
		Tools:  []agents.ToolGrant{{Name: "whoami"}, {Name: "elsewhere"}, {Name: "write"}, {Name: "read"}, {Name: "git_refs_demo", Allowlist: map[string][]string{"repo": {"acme/app"}}}},
	})
	if err != nil {
		return err
	}
	if runCommands {
		input := "whoami {\"who\":\"me\"}\nelsewhere {}\nwrite {\"text\":\"hello from the sandbox\"}\nread {}"
		desc := `Demo 9 of 10: commands in a sandbox never see the credential. A command's environment holds a placeholder where the credential goes, plus its arguments and the egress URL, nothing else; its requests go out through egress, which exchanges a live placeholder for the real value, for the tool's hosts only, and retires it when the call ends.
whoami: "env: fake_…" is the whole credential the command had. It sent the placeholder to egress, which swapped in the real value; the echo service reports the header it received with the placeholder put back by egress, so the real value never came back into the sandbox.
elsewhere: the same placeholder sent for another host. Egress answers 403 and nothing reaches that host: the placeholder is bound to the hosts its tool declared.
write then read: files persist across a run's commands; another run would not see note.txt. The sandbox is released when the run ends.`
		if _, err := exec.LookPath("git"); err == nil {
			input += "\ngit_refs_demo {\"repo\":\"acme/app\"}\ngit_refs_demo {\"repo\":\"evil/app\"}"
			desc += `
git_refs_demo: the real git, holding only a placeholder as its password, against a fake git server built into the API that lists its branches only to the real stored value (a placeholder gets 401, as GitHub would). Seeing refs/heads/real-credential-received proves egress found the placeholder inside git's Basic credential and swapped it. The second git_refs_demo is refused by the allowlist (repo: acme/app) before anything runs.`
		}
		if _, err := run(executions.Run{AgentID: ag.ID, Input: input, Description: desc}); err != nil {
			return err
		}
	}

	// 10. gh through TLS interception, in Docker with a real token.
	github, err := create(agents.Definition{
		Name: "10-gh-tls-intercept", Model: "mock", Instructions: "Work with GitHub through gh and git.",
		Budget: agents.Budget{MaxCalls: 10},
		Tools:  []agents.ToolGrant{{Name: "gh_prs"}, {Name: "gh_direct"}, {Name: "gh_elsewhere"}, {Name: "git_refs"}},
	})
	if err != nil {
		return err
	}
	if githubToken != "" && o.RunGitHub {
		// In the background: each step is a container and a call to GitHub,
		// and gh_direct waits out a timeout if the sandbox has no route to
		// the internet; the server should not.
		go func() {
			r := executions.Run{
				AgentID: github.ID,
				Input:   "gh_prs {\"repo\":\"cli/cli\"}\ngh_elsewhere {}\ngit_refs {\"repo\":\"cli/cli\"}\ngh_direct {}",
				Description: `Demo 10 of 10: the real gh in a Docker sandbox, holding only a placeholder. gh always calls https://api.github.com, so the sandbox has egress as its HTTPS proxy and trusts the platform CA; egress answers gh's CONNECT with a certificate for api.github.com, swaps the placeholder for the token inside the TLS connection, and makes the real call. The token never enters the sandbox.
gh_prs: "env: fake_…" is all gh had; then your login and three open pull requests, real answers from GitHub. The server log shows "egress connect host=api.github.com" and "swapped=true".
gh_elsewhere: gh pointed at another host with the same placeholder. Egress answers 403 before anything leaves.
git_refs: the same token through the other form of egress, a URL prefix, with the real git: the branch list from github.com.
gh_direct: the placeholder sent straight to GitHub, around egress: "Bad credentials". Outside egress it is worthless. (It gives up after 15 seconds if the sandbox has no route to the internet.)
Retry any step: "replayed: nothing ran", the saved result, no container started. From your shell, the placeholder replayed through the proxy gets 401: it was retired when its call ended.`,
			}
			e, err := a.executions.Start(ctx, tenant, r)
			if err != nil {
				a.log.Error("seeded github run failed to start", "tenant", tenant, "err", err)
				return
			}
			a.log.Info("seeded github run finished", "tenant", tenant, "execution", e.ID, "status", e.Status)
		}()
	}
	return nil
}
