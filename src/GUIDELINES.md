# Guidelines

How to write code here. Every rule carries its reason; a rule without one gets dropped.

## The deciding factor

A domain folder can be dragged into another project and work out of the box. When two designs are defensible, the more portable one wins: it keeps each part replaceable, testable and understandable alone.

## Domain shape

```
src/server/internal/<domain>/
  <domain>.go     what it is, its errors, API and constructor
  types.go        its structs and constants
  deps.go         interfaces it needs
  logic.go        pure logic, with logic_test.go
  service.go      I/O and sequencing, with service_test.go
  deps/memory/    in-memory adapter, shipped with the domain
  deps/sqlite/    real adapter, with its own migrations
```

- `deps.go` declares, `deps/` implements, so the domain never depends on what sits behind an interface.
- `deps/memory/` ships with the domain, so a moved domain runs and passes its tests at once.
- Domains never import each other, so each can move alone; each declares its own errors.

## Functional core, imperative shell

- `logic.go` is pure: no ctx, I/O, clock, randomness or globals. It is tested as a table, with no mocks.
- `service.go` only sequences; any decision belongs in `logic.go`, where it can be tested.
- No global state, `init()` or singletons: each ties a domain to one process.
- Clock, IDs, config and loggers are passed in; never `time.Now()` or `os.Getenv` in a domain, so tests are deterministic and a new host needs no hidden setup.

## Errors

- Wrap with `%w` and context, to find failures without a debugger.
- Never panic across a domain boundary; the caller cannot contain it.
- `ctx` comes first on every I/O call, and cancellation is honoured, so nothing hangs.

## Tests

- Tests sit beside the file. Logic: table-driven. Services: against `deps/memory`, never a mocking framework.
- A domain's tests pass with only its own folder present.
- Retries, timeouts and backoff use `testing/synctest` (`GOEXPERIMENT=synctest` on Go 1.24) for a fake clock.

## UI

The same rules, per domain in `src/ui/src/domains/<domain>/`: components are pure functions of props, a provider injects deps, domains never import each other, state is `readonly`, and styling uses semantic theme tokens only.
