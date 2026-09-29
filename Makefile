.PHONY: check test fmt run run-sandbox sandbox-image ui

check: src/ui/node_modules
	cd src/server && test -z "$$(gofmt -l .)" && go vet ./... && go test ./...
	cd src/ui && npm run typecheck

test:
	cd src/server && go test ./...

fmt:
	gofmt -w src/server

# The API on :8080, egress on :8081 and the UI on :5173, all in memory and
# seeded for acme; sandboxed commands run as local processes. Every demo but
# the gh one (which needs Docker: run-sandbox). Ctrl-C stops both.
run: src/ui/node_modules .build/server ports-free
	@echo
	@echo "  Open http://localhost:5173 and sign in as acme."
	@echo
	@$(call both,./.build/server)

# The same with the Docker sandbox on Colima, for the gh demo (README demo 4):
#   GH_CLI_TOKEN=$$(gh auth token) make run-sandbox
# The token is stored as the credential github_token and a `github` agent is
# seeded with the gh tools; the sandbox only ever holds a placeholder for it.
# Colima must run with an address (colima start --network-address): the Mac
# is then 192.168.64.1 from a container, which EGRESS_URL overrides.
run-sandbox: src/ui/node_modules .build/server ports-free
	@colima status 2>&1 | grep -q 'address:' || { echo "Colima has no address: run 'colima start --network-address'"; exit 1; }
	@docker image inspect aof-sandbox >/dev/null 2>&1 || $(MAKE) sandbox-image
	@test -n "$$GH_CLI_TOKEN" || echo "  GH_CLI_TOKEN is not set: store a token as github_token in the Tools tab before running the gh tools."
	@echo
	@echo "  Open http://localhost:5173, sign in as acme, run the agent 'github' with: gh_prs {\"repo\":\"cli/cli\"}"
	@echo
	@$(call both,env SANDBOX=docker EGRESS_URL=$${EGRESS_URL:-http://192.168.64.1:8081} ./.build/server)

# Refuse to start on top of an earlier run: the server would fail to bind
# and only the UI would come up.
ports-free:
	@for p in $${ADDR:-:8080} $${EGRESS_ADDR:-:8081} :$${PORT:-5173}; do \
	  pid=$$(lsof -nP -iTCP:$${p#:} -sTCP:LISTEN -t 2>/dev/null | head -1); \
	  if [ -n "$$pid" ]; then echo "port $${p#:} is in use by pid $$pid ($$(ps -p $$pid -o command= | cut -c1-60)); stop it first (an earlier make run?)"; exit 1; fi; \
	done

# Runs the server ($(1)) and the UI dev server together, in the background,
# and stops both when either exits or on Ctrl-C.
both = trap 'kill $$server $$ui 2>/dev/null' EXIT INT TERM; \
	$(1) & server=$$!; \
	(cd src/ui && exec npm run dev) & ui=$$!; \
	while kill -0 $$server 2>/dev/null && kill -0 $$ui 2>/dev/null; do sleep 1; done

# The UI dev server alone, proxying /v1 to API_URL or localhost:8080.
ui: src/ui/node_modules
	cd src/ui && npm run dev

# The Docker sandbox image: a shell, curl, git and the GitHub CLI.
sandbox-image:
	docker build -t aof-sandbox src/server/internal/sandbox/deps/docker

# Built rather than `go run`, so stopping make stops the server itself.
.build/server: $(shell find src/server -name '*.go') src/server/go.mod
	cd src/server && go build -o ../../.build/server ./cmd/server

src/ui/node_modules: src/ui/package-lock.json
	cd src/ui && npm ci && touch node_modules
