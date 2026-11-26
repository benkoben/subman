# Structure the HTTP service around explicit composition

- **Status:** Accepted
- **Date:** 2026-11-26

## Context

Mat Ryer's [How I write HTTP services in Go after 13 years](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/) recommends a small set of explicit, standard-library-oriented patterns: a `NewServer` constructor returning `http.Handler`, a complete `routes.go`, handler maker functions with explicit dependencies, a minimal `main` calling an injectable `run`, centralized JSON helpers, middleware as `http.Handler` adapters, and tests that exercise the program through its real HTTP surface. The [research notes](../research/grafana-http-service-practices.md) record the article's recommendations and caveats.

Subman already follows some of these practices. `api.NewHandler` returns an `http.Handler`, uses Go's method-and-path `ServeMux` patterns, and retains the existing functional-option clock seam. `json.go` centralizes strict request decoding and response encoding. API tests use external test packages, real requests and response recorders, the complete mux, and memory repositories rather than testing every handler in isolation. The runner already connects to MongoDB before serving and gracefully shuts down HTTP and MongoDB.

The main gaps are structural and at the process boundary:

- `internal/api/handler.go` contains every route, handler closure, response mapping, and transport type, so there is no concise route map and handler dependencies are implicit in one large closure scope.
- `main` has a `run` function, but `run` reads the process environment and writes through the package-level logger directly. Its use of `ListenAndServe` also gives tests no deterministic way to pre-bind and discover an ephemeral listener address.
- There is no readiness endpoint or test that starts and stops the complete program through `run`.
- There is no top-level middleware composition point distinct from route registration.

A literal, mechanical adoption of every example would conflict with more important repository boundaries. Handlers must continue to depend on `app.Service`, the only layer that coordinates aggregates and enforces ownership; they must not receive repositories directly. Domain invariants must remain in domain constructors and lifecycle methods rather than move into transport validation. Shared response types are genuinely reused by create, list, and Profile responses, and the current service has no expensive handler initialization that would justify `sync.Once`.

This is a coherent architectural choice about HTTP composition, process lifecycle, and the testing boundary. It will shape future handlers and tests, is costly to reverse after more endpoints accumulate, and trades additional files and explicit arguments for discoverability and testability.

## Requirements

- Preserve the existing HTTP routes, JSON representations, status codes, strict unknown-field and trailing-value rejection, ownership rules, and per-response clock semantics, except for the deliberate addition of a health endpoint.
- Keep `app.Service` as the only coordination and ownership-enforcement layer between HTTP and repositories.
- Make the entire API surface visible in one route-registration file.
- Construct handlers as `http.Handler` values with explicit arguments, without introducing a server struct containing hidden dependencies.
- Keep `main` limited to supplying real operating-system dependencies and translating a returned error into process exit behavior.
- Make `run` deterministic and testable without global environment mutation, global loggers, fixed test ports, or `os.Exit` below `main`.
- Preserve bounded startup and graceful HTTP/MongoDB shutdown, including `SIGINT` and `SIGTERM` handling and the existing timeouts.
- Centralize JSON encoding and strict decoding while leaving domain validation in the application/domain layers.
- Prefer tests through the complete HTTP surface, avoid duplicative handler-unit tests, and add a real-MongoDB `run` test without making the ordinary unit/integration suite require a running database.
- Do not invent middleware, validation frameworks, lazy initialization, CLI arguments, or additional dependencies merely to reproduce examples in the article.
- Update repository guidance and user-facing documentation with the resulting structure and test workflow. Domain terminology and rules do not change, so `CONTEXT.md` must remain unchanged.

## Considered Options

### Adopt the article's composition patterns with Subman-specific boundaries

Use `NewServer`, `routes.go`, maker handlers, centralized generic JSON helpers, and an injectable `run`. Keep the application service and domain validation boundaries, retain useful shared response DTOs, and apply conditional techniques such as middleware and `sync.Once` only when a real need appears. Use existing memory-backed API tests for broad behavior coverage and an opt-in real-MongoDB test for the full process path.

This option is preferred because it captures the article's maintainability and testability benefits without bypassing aggregate coordination or adding speculative machinery.

### Copy every article example literally

Pass individual stores into the API, add the article's `Validator` interface to all request DTOs, inline all endpoint types, introduce `sync.Once`, and make all tests launch `run` against MongoDB.

This option is rejected because it would weaken Subman's application boundary, duplicate or relocate domain validation, discard useful response reuse, add unused code, and make routine tests slower and dependent on external infrastructure. The article itself presents its testing and lazy-initialization advice as context-dependent rather than universal.

### Retain the current structure and only add isolated tests

Keep the single `handler.go` and the current process runner while adding a health endpoint or narrowly targeted runner tests.

This option is rejected because it leaves the main discoverability and dependency-injection gaps in place. It would make future handlers continue to accumulate in one closure and would not establish the requested conventions for future work.

## Decision

Adopt the article's explicit HTTP composition and runner patterns, adapted to Subman's existing application and domain boundaries.

### HTTP construction and routing

- Rename `api.NewHandler` to `api.NewServer`. It will retain the existing `Option`/`WithClock` behavior, create a private `http.ServeMux`, call `addRoutes`, apply future global middleware around that mux, and return an `http.Handler`.
- Put every route, including the new health route, in `internal/api/routes.go`. Route registration will pass only the dependencies each handler needs.
- Replace anonymous route closures with `handle...` maker functions returning `http.Handler`. These functions receive `*app.Service` and, only where derived state requires it, the clock function. Repositories remain behind the application service.
- Split handlers by API resource and keep response mapping/shared response DTOs in a separate response file. Define request DTOs inside their maker function when only that endpoint uses them; do not inline genuinely shared response types.
- Retain one JSON helper file with generic decoding and encoding. Decoding remains strict about unknown fields and trailing JSON values, and error responses remain `{"error":"..."}`. Required-field and UUID checks stay at the transport edge; business invariants and ownership checks stay in their current layers.
- Do not add a `Validator` interface in this refactor. A future transport-only `Valid` pattern is acceptable only if it preserves the public error contract and does not hide database-backed or domain validation.
- Do not add unused middleware. When needed, global middleware belongs in `NewServer`, route-specific wrapping belongs in `routes.go`, and dependency-heavy middleware should use a function returning `func(http.Handler) http.Handler`.
- Do not introduce `sync.Once` without concrete expensive handler setup. If later used, first-request latency, cached initialization failure, concurrency, and tests must be considered explicitly.

### Program runner and lifecycle

- Reduce `main` to calling `run(context.Background(), os.Getenv, os.Stderr, net.Listen)`, printing a returned error to standard error, and exiting non-zero. `log.Fatal` and process exits below `main` are not used.
- Give `run` explicit context, environment lookup, error writer, and listener-factory parameters. Do not add unused `args`, standard input, or standard output parameters until the program actually supports them.
- Install `signal.NotifyContext` inside `run` and defer its stop function. Construct a scoped logger from the injected writer rather than using the package-level logger.
- Resolve configuration only through the injected environment function. Use the listener factory and `http.Server.Serve` so tests can supply an already-bound `127.0.0.1:0` listener and know its actual address without a free-port race.
- Preserve MongoDB ping-before-serve construction, startup/shutdown timeouts, `ReadHeaderTimeout`, request contexts, `http.ErrServerClosed` handling, and MongoDB disconnection. Cancellation must stop the server and wait for the serving goroutine to finish before `run` returns.

### Testing and readiness

- Add `GET /healthz`, returning `200` with `{"status":"ok"}` after the server is running. MongoDB connectivity has already been verified before serving, so this endpoint is a lightweight liveness/readiness signal, not a new database-checking subsystem.
- Keep the existing API tests as the main behavior suite, updated to construct `api.NewServer`. They already exercise routing, decoding, handlers, application use cases, and response encoding through the public HTTP surface. Add focused health and middleware tests only when those paths exist; do not add per-handler duplicates.
- Add a `main` package integration test that launches `run` with a pre-bound listener, an isolated real MongoDB database, and captured logs. Poll `/healthz` with a bounded context, exercise a representative HTTP request, cancel the run context, and assert bounded clean shutdown.
- Enable that process-level test only when `SUBMAN_TEST_MONGODB_URI` is set; otherwise skip it with a clear reason. Use a unique test-owned database and clean up only that database. Ordinary `go test ./...` remains self-contained, while the full production path can be verified explicitly with the test URI.

## Consequences

- The route surface and each handler's dependencies become visible without reading a 500-line constructor. New endpoints have a clear, repeatable location and shape.
- `main` can be exercised with controlled environment, logs, cancellation, and network binding. Tests do not need `t.Setenv`, fixed ports, or global logger redirection.
- The API package gains several small files and repeated explicit `svc`/`clock` arguments. This is intentional boilerplate in exchange for local reasoning and compile-time dependency checking.
- Existing callers must rename `NewHandler` to `NewServer`; no compatibility wrapper is needed because this is an internal package and all call sites are in this repository.
- The public API gains only `/healthz`; existing resource contracts and domain behavior remain unchanged.
- Fast tests continue to use memory repositories, so they do not prove MongoDB wiring. The opt-in `run` integration test covers that gap when a test database is supplied, but its skip status must be reported honestly when it is not run.
- Future middleware, transport validation, and lazy initialization have established places and constraints without imposing unused abstractions now.
- No new third-party dependency, generated artifact, domain term, or domain rule is introduced.

## Implementation Plan

1. Obtain explicit user acceptance of this ADR, then change its status and the index entry to `Accepted` before changing implementation code.
2. Refactor `internal/api` without changing existing resource behavior:
   - move constructor options and `NewServer` into `server.go`;
   - list all current resource routes in `routes.go`;
   - extract resource maker handlers into `users.go`, `cards.go`, `subscriptions.go`, and `profile.go`;
   - move shared response DTOs and mapping functions into `responses.go`, keeping endpoint-only request DTOs local to their handlers;
   - replace `decodeJSON`/`writeJSON` with generic `decode`/`encode` helpers while retaining strict decoding and the existing error writer;
   - update `main.go` and all API tests to call `NewServer`, removing `handler.go` once its contents have been moved.
3. Refactor the process boundary in `main.go`:
   - implement the minimal `main` and injectable `run` signature described above;
   - replace direct environment/global logger access with the injected functions and scoped logger;
   - introduce listener injection and `Serve`, preserving the current server settings, startup timeout, signal handling, shutdown timeout, and MongoDB cleanup;
   - ensure both server failure and context cancellation paths wait for the serving goroutine and return wrapped, meaningful errors.
4. Add readiness and process-level tests:
   - register and test `GET /healthz` in the API suite;
   - add `main_test.go` helpers for a pre-bound ephemeral listener, environment map, and unique MongoDB test database cleanup; make `/healthz` polling apply its deadline and backoff to transport errors as well as non-200 responses;
   - add the opt-in real-MongoDB `run` integration test covering startup, a representative API request, cancellation, and clean termination, explicitly capturing and asserting the error returned by `run`;
   - retain the existing API tests as the primary behavior regression suite and avoid duplicating their cases at the process level.
5. Update documentation in the same changeset:
   - revise `README.md` architecture for `NewServer`, `routes.go`, maker handlers, and the injectable runner; document `/healthz` and how to run the opt-in process integration test;
   - update `AGENTS.md` repository map from `NewHandler` to `NewServer` and describe the listener/environment injection seam;
   - leave `CONTEXT.md`, the domain packages, repositories, and application service unchanged unless verification exposes a pre-existing defect, which must be reported separately rather than folded into this refactor.
6. Review the final route map against `README.md`, inspect the diff for accidental behavior changes or speculative patterns, and run the verification below.

## Verification Plan

- Run `gofmt` on every changed Go file, then run `go test ./...`, `go vet ./...`, and `go build ./...` from the repository root.
- Run `SUBMAN_TEST_MONGODB_URI=<test-only-uri> go test .` when a disposable MongoDB deployment is available; otherwise record that the process-level integration test was skipped and do not claim that MongoDB wiring was verified.
- Confirm all pre-existing API tests still verify the same status codes, JSON contracts, strict decoding, ownership behavior, derived-state behavior, and single-clock-read semantics.
- Confirm `routes.go` contains every documented application route plus `GET /healthz`, and that `NewServer` returns the fully composed `http.Handler`.
- Confirm no handler receives a repository and no domain validation moves into transport DTOs.
- Confirm `run` tests use no process environment mutation or fixed ports, `/healthz` polling has a deadline, test database cleanup is scoped to the unique test database, and cancellation causes `run` to return within a bounded time.
- Confirm normal operation still handles `SIGINT`/`SIGTERM`, startup and shutdown remain bounded, and the serving goroutine finishes before return.
- Run `git diff --check` and inspect the final diff for unrelated edits, generated binaries, and accidental changes to `CONTEXT.md`.
