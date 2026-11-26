# Mat Ryer's HTTP service practices: research note

**Purpose:** sourced input for an architecture decision, not a decision or implementation plan.  
**Primary source:** Mat Ryer, [“How I write HTTP services in Go after 13 years”](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/), Grafana Labs, 2024-02-10.  
**Research date:** 2026-11-26.

Part I reports Ryer's recommendations, including his qualifications. Standard-library cross-checks are separately labeled. Part II contains **my own applicability observations** from read-only inspection of Subman; they are not Ryer's recommendations.

## Part I — Practices recommended by the article

### 1. Build the service around `NewServer` and `http.Handler`

- Use one `NewServer` constructor per service. Pass all dependencies explicitly, construct its mux, call the route-registration function, wrap the mux in service-wide middleware (for example CORS, authentication, tracing, and logging), and return an `http.Handler` where possible; a dedicated handler type is reserved for more complex cases. Ryer has moved away from handler methods on a server struct because that conceals dependencies from someone testing one handler. ([NewServer](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#the-new-server-constructor), [changes from the earlier article](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#who-is-this-post-for))
- Prefer `http.Handler` as the usual representation because third-party libraries commonly target it. `http.HandlerFunc` remains useful, and Ryer says the practical difference is small. ([Changes](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#who-is-this-post-for))
- Long dependency lists are acceptable and can be formatted vertically. Ryer values function arguments over a dependency struct because callers cannot omit required arguments, whereas struct fields can be left unset. He acknowledges that there must be a length at which this stops being appropriate. Tests that do not exercise a dependency may pass `nil` as an explicit signal that it must not be used. ([Long argument lists](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#long-argument-lists))

**Stdlib cross-check:** `http.Handler` is the one-method `ServeHTTP(ResponseWriter, *Request)` interface; `http.HandlerFunc` is the adapter that lets a function implement it. Thus returning the interface does not discard the convenience of function handlers. ([`net/http.Handler`](https://pkg.go.dev/net/http#Handler), [`net/http.HandlerFunc`](https://pkg.go.dev/net/http#HandlerFunc))

### 2. Map the API surface in `routes.go`

Keep every route in one `routes.go` where practical, so a reader can discover the service's entire HTTP surface in one place. The route function normally repeats `NewServer`'s dependency list; Go's type checking helps detect omitted or incompatible arguments. Ryer normally makes `addRoutes` infallible by doing error-producing setup earlier in `run`, but permits it to return an error when handler construction genuinely can fail. ([`routes.go`](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#map-the-entire-api-surface-in-routesgo))

### 3. Make `main` only call an OS-injected `run`

- Keep `main` minimal and put the program in `run`, which accepts operating-system fundamentals and returns an error. `main` supplies the real context, arguments, streams, and environment function; it prints a returned error to stderr and exits nonzero. If exact CLI exit codes matter, Ryer would also return an integer so tests can assert it. ([`main`/`run`](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#func-main-only-calls-run))
- Candidate injected values are `os.Args` (`[]string`), stdin (`io.Reader`), stdout/stderr (`io.Writer`), `os.Getenv` (`func(string) string`), and `os.Getwd` (`func() (string, error)`), selected as needed. Tests can then invoke the real program while controlling arguments, streams, working-directory lookup, and environment without process globals. Avoiding global state also makes more calls to `t.Parallel` safe. ([`main`/`run`](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#func-main-only-calls-run))
- Create the signal-derived context **inside `run`**, with its cancellation/stop function deferred there. Ryer moved this out of `main` after readers pointed out that the error path's `os.Exit` would prevent a defer in `main` from running. ([`main`/`run`](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#func-main-only-calls-run))

**Stdlib cross-check:** `signal.NotifyContext` cancels its child context on a listed signal, parent cancellation, or invocation of its `stop` function; callers should invoke `stop` to release resources and restore signal behavior. `os.Exit` terminates immediately and does not run deferred functions. ([`os/signal.NotifyContext`](https://pkg.go.dev/os/signal#NotifyContext), [`os.Exit`](https://pkg.go.dev/os#Exit))

### 4. Propagate cancellation and shut down gracefully

Pass the signal-cancellable context through every layer, at least into dependencies and preferably into long-running or looping code that checks `ctx.Err()`/`ctx.Done()`. Use it to stop extra goroutines too. Ryer starts the HTTP server alongside a shutdown path with a bounded `http.Server.Shutdown` call and treats `http.ErrServerClosed` as the expected serving result. Reliable shutdown matters particularly when each test starts a program instance, although sharing one instance across tests is presented as a valid alternative. ([Graceful shutdown](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#gracefully-shutting-down), [NewServer example](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#the-new-server-constructor))

**Stdlib cross-check:** `Shutdown` closes listeners and idle connections, then waits for active connections to become idle until the supplied context expires; it does not interrupt active handlers itself. `ListenAndServe` returns `ErrServerClosed` after `Shutdown` or `Close`. Hijacked connections are not closed or waited on by `Shutdown`. ([`http.Server.Shutdown`](https://pkg.go.dev/net/http#Server.Shutdown), [`http.ErrServerClosed`](https://pkg.go.dev/net/http#ErrServerClosed))

### 5. Inject flags and environment rather than mutate process state

Parse flags from the injected args with a local `flag.NewFlagSet` inside `run`, not the package-global flag set. Inject `getenv` so tests can supply values with a function instead of changing the actual environment. Ryer prefers this to `t.Setenv`, specifically because environment mutation prevents parallel tests; flags, environment variables, or both remain supported. ([Controlling the environment](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#controlling-the-environment))

**Stdlib cross-check:** current `testing.T.Setenv` documentation says it affects the whole process and cannot be used in a parallel test or one with parallel ancestors, confirming Ryer's parallelism caveat. ([`testing.T.Setenv`](https://pkg.go.dev/testing#T.Setenv))

### 6. Use maker functions that return handlers

A handler maker accepts its dependencies and returns an `http.Handler` (often by returning an `http.HandlerFunc`), rather than itself being the request function. Its closure can perform setup once and expose the resulting values to every request. Captured data should be read-only; mutation needs synchronization such as a mutex. Ryer cautions against treating this closure as durable application state because cloud instances stop, crash, and sit behind load balancers; real state belongs in a database or another storage API. ([Maker functions](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#maker-funcs-return-the-handler))

### 7. Centralize generic JSON encoding and decoding

Use small generic `encode[T]` and `decode[T]` helpers to wrap the repeated JSON mechanics and error context. Ryer's encoder sets `Content-Type: application/json`, writes the requested status, and calls `json.Encoder.Encode`; his decoder reads the request body with `json.Decoder.Decode`. The abstraction is only a few lines and would not ordinarily be necessary, but centralization pays off when cross-cutting behavior must change for every endpoint (his deliberately humorous example is adding XML). He advises against overloading these helpers with many responsibilities. Type inference lets callers omit `T` for `encode`, while `decode[RequestType](r)` must specify the result type. ([Encoding/decoding](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#handle-decodingencoding-in-one-place))

**Stdlib cross-check:** `Decoder.Decode` reads the **next** JSON value from a stream, so one call alone neither rejects a second value nor enables unknown-field rejection; `DisallowUnknownFields` is a separate option. `Encoder.Encode` appends a newline. These details matter when comparing the article's minimal helpers with stricter API policies. ([`json.Decoder.Decode`](https://pkg.go.dev/encoding/json#Decoder.Decode), [`json.Decoder.DisallowUnknownFields`](https://pkg.go.dev/encoding/json#Decoder.DisallowUnknownFields), [`json.Encoder.Encode`](https://pkg.go.dev/encoding/json#Encoder.Encode))

### 8. Add lightweight `Valid` validation to decoded request objects

Ryer likes a single-method interface of the form `Valid(context.Context) map[string]string`. The context is optional in principle but has proved useful; map keys identify fields and values contain human-readable problems, and zero problems means valid. A nil map is safe because `len(nilMap) == 0`. Appropriate checks include required fields, string formats such as email, and numeric ranges. Database-backed or similarly significant validation belongs elsewhere so it does not become hidden inside a superficially simple check. Integration can be dynamic through a type assertion or explicit through `decodeValid[T Validator]`; the latter decodes, calls `Valid(r.Context())`, and returns the problems plus an error when any exist. ([Validation](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#validating-data))

### 9. Represent middleware as `http.Handler` adapters

Use functions that accept the next `http.Handler` and return another one. Middleware may execute before and/or after the next handler, or short-circuit without calling it; Ryer's `adminOnly` example returns 404 to a non-admin, while an admin proceeds. Applying adapters at their registrations in `routes.go` makes middleware coverage visible in the API map, with multiline formatting as compositions grow. ([Middleware adapters](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#the-adapter-pattern-for-middleware))

### 10. Use a middleware factory when dependencies make adapters repetitive

For simple middleware, use the adapter directly. If every registration would repeat logger, database, clients, and other dependencies, construct the middleware once with a factory returning `func(http.Handler) http.Handler`, then apply that configured adapter to each route. A named `middleware` function type is optional; Ryer avoids it because the explicit function signature is clearer to readers and adds less indirection. His stated priority is ease of reading, not ease of writing. ([Middleware factories](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#sometimes-i-return-the-middleware))

### 11. Keep endpoint-specific request/response DTOs local

When request and response structures belong only to one endpoint, declare them inside its handler maker. This keeps package scope clear and prevents other handlers from coupling to an endpoint shape that may not be stable. If tests or other code genuinely need the same types, Ryer accepts that friction as a good reason to move them out; locality is not an absolute rule. ([Local DTOs](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#an-opportunity-to-hide-the-requestresponse-types-away))

### 12. Use test-local DTOs to tell the endpoint's story

Even when production code uses a broad shared type, tests can declare a small inline/anonymous structure containing only the JSON fields relevant to the interaction. Besides avoiding dependence on hidden production DTOs, this documents for future readers exactly which fields the endpoint and test care about—Ryer illustrates a greeting test that includes only `Name` from a broader `Person` concept. ([Test storytelling](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#use-inline-requestresponse-types-for-additional-storytelling))

### 13. Defer genuinely expensive handler setup with `sync.Once`

If preparing a handler requires expensive work such as parsing templates, Ryer moves it into `sync.Once.Do` on the first request. This improves startup, avoids all work for an unused endpoint, and makes concurrent callers wait for the single initialization. Keep the error check outside the `Do` callback so the cached failure is surfaced to requests rather than disappearing into logs. The trade-off is explicit: initialization latency moves from startup to the first request, and a failure is retained rather than retried. Ryer says this suits his App Engine deployments, not every deployment. ([`sync.Once`](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#synconce-to-defer-setup))

**Stdlib cross-check:** `Once.Do` runs only the first invocation's function, and no call returns until that function returns. A panic counts as completion, so later calls do not retry; a `Once` must not be copied after first use. ([`sync.Once.Do`](https://pkg.go.dev/sync#Once.Do))

### 14. Optimize the test suite for understanding and confidence, favoring API-level end-to-end tests

Ryer evaluates testing by three questions: do tests explain program behavior, can code change without fear, and would passing tests provide enough confidence to deploy? Handlers remain independently testable by constructing one with explicit dependencies, invoking `ServeHTTP` with a real request and `httptest.ResponseRecorder`, and asserting status, decoded body, and important headers. That approach is useful for isolated complexity but bypasses middleware and the route map. ([Testing philosophy](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#designing-for-testability), [Choosing the unit](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#what-is-the-unit-when-unit-testing))

His default is therefore to call `run` and exercise the API as users do: parse real arguments, connect to dependencies, perform migrations or other startup work, start the server, use a real database, and traverse routes and middleware. He reports catching more issues earlier and avoiding repeated assertions about every layer. He still uses focused unit tests where appropriate, including tests produced through TDD, but deletes tests that merely duplicate end-to-end coverage. This is explicitly contextual advice based on project complexity and team judgment, not a prohibition on unit tests. ([Choosing the unit](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#what-is-the-unit-when-unit-testing))

**Stdlib cross-check:** `httptest.ResponseRecorder` is explicitly an implementation of `http.ResponseWriter` for recording a handler's result in tests. ([`httptest.ResponseRecorder`](https://pkg.go.dev/net/http/httptest#ResponseRecorder))

### 15. Run a self-contained program per test and clean it up through context

Ryer likes each test to invoke `run` with its own args, flags, streams, and environment. Create a cancellable context and register cancellation with `t.Cleanup` (introduced in Go 1.14), so completion of the test cancels the program and triggers graceful shutdown. This depends on the implementation consistently observing context cancellation. ([Testing with `run`](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#testing-with-the-run-function))

**Stdlib cross-check:** cleanup functions run after the test/subtest and its subtests complete, in last-added, first-called order. Current Go also provides `T.Context`, canceled immediately before cleanup callbacks, but the article's `context.WithCancel` plus `t.Cleanup(cancel)` remains valid. ([`testing.T.Cleanup`](https://pkg.go.dev/testing#T.Cleanup), [`testing.T.Context`](https://pkg.go.dev/testing#T.Context))

### 16. Poll a real readiness endpoint before end-to-end requests

Because `run` starts asynchronously, tests need a readiness barrier. Ryer mentions a channel as an option but prefers an official `/healthz` or `/readyz` endpoint: polling with actual HTTP requests proves the externally visible service is ready and also gives users/operators a useful readiness mechanism. His illustrative helper repeatedly makes a context-aware GET, treats 200 as ready, closes response bodies, pauses 250 ms between checks, and is intended to stop on cancellation or a caller-supplied timeout. ([Readiness](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#waiting-for-readiness))

### 17. Prefer explicit, maintainable patterns, but adapt to the organization

Ryer's broader goal is readable, extendable, explicitly wired code without “magic,” whose patterns newcomers can copy safely; code generation can still produce that boilerplate. On larger projects, established technologies and team experience—gRPC is his example—can outweigh these personal defaults, and he recommends pragmatically following established abstractions rather than forcing this shape everywhere. ([Putting it into practice](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#putting-this-all-into-practice))

## Part II — My applicability observations for Subman

These are repository observations and source-critical caveats, **not additional recommendations from Ryer**.

1. **Several structural ideas are already present, under different names.** `api.NewHandler` already returns `http.Handler`, owns a private `http.ServeMux`, and receives a composed application `Service` plus an injectable clock; all routes are visible in that one constructor in `internal/api/handler.go`, rather than a separate `routes.go`. Thus the article's constructor and single-API-map ideas are distinguishable from its particular `NewServer` name, long flat dependency list, and file layout. `main.go` likewise already has an error-returning `run`, signal-derived cancellation for SIGINT/SIGTERM, bounded HTTP shutdown, and MongoDB disconnect. Its `run` does **not** inject args, streams, or `getenv`, which is the principal difference from the article's testability seam. (`internal/api/handler.go`, `main.go`; [article constructor](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#the-new-server-constructor), [article `run`](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#func-main-only-calls-run))

2. **Subman already has central JSON helpers, with a stricter decoding contract than the article's example.** `decodeJSON` rejects unknown fields and trailing JSON values, while `writeJSON` centralizes content type, status, and encoding. The production DTOs are package-scoped and shared where responses overlap; tests already declare independent, often minimal response shapes, closely matching the article's storytelling rationale. `writeJSON` currently discards encoding errors, whereas Ryer's helper returns them; independently of that difference, his example writes the status before encoding, so an encoding failure cannot then replace the status line. (`internal/api/json.go`, `internal/api/handler.go`, `internal/api/handler_test.go`, `internal/api/cards_test.go`; [`http.ResponseWriter.WriteHeader`](https://pkg.go.dev/net/http#ResponseWriter))

3. **Validation has two existing layers that the `Valid` pattern should not blur in an ADR.** HTTP handlers check transport concerns such as required timestamp fields and UUID syntax; domain constructors/application use cases enforce invariants and ownership. Ryer's `Valid` interface is aimed at lightweight request-field validation, and he explicitly excludes database-backed checks, so it does not account for Subman's domain and ownership rules by itself. (`internal/api/handler.go`, `internal/app`, domain constructors; [article validation scope](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#validating-data))

4. **Middleware and lazy setup currently have little direct surface area in the inspected API.** The handler registers endpoints without a middleware chain, and there is no evident template parsing or comparably expensive per-handler setup. Consequently, the article supplies well-sourced patterns and concurrency caveats for those situations, but the current code does not itself demonstrate a concrete `sync.Once` use case. (`internal/api/handler.go`; [article middleware](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#the-adapter-pattern-for-middleware), [article lazy setup](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#synconce-to-defer-setup))

5. **Current tests sit between Ryer's isolated-handler and full-`run` examples.** They construct the complete mux and application service with in-memory repositories, then invoke `ServeHTTP` through `httptest`; this exercises routes and use cases but not `main`, OS configuration, signals, a real listener, or MongoDB. Ryer's full-`run` approach would add those production layers at the cost of external-process/resource coordination, while the present suite retains deterministic clock and repository seams. This is a coverage/trade-off distinction, not evidence that either style is universally superior—Ryer himself qualifies the choice. (`internal/api/handler_test.go`, `internal/api/subscriptions_test.go`, `main.go`; [article testing choice](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#what-is-the-unit-when-unit-testing))

6. **No readiness route is currently registered or documented.** The README lists the domain API but no `/healthz` or `/readyz`; therefore the article's readiness barrier describes a capability absent from the inspected service, not an existing test convention. (`internal/api/handler.go`, `README.md`; [article readiness](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#waiting-for-readiness))

7. **Two article snippets need care if used as evidence rather than copied literally.** In the readiness helper, the `continue` after a transport error occurs before the cancellation/timeout `select`, so persistent connection errors can bypass the stated stopping conditions and retry without the intended delay. The minimal test snippet also launches `go run(ctx)` without observing the returned error. These are my readings of the article's illustrative code, not additional practices to adopt. ([Readiness code](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#waiting-for-readiness), [`run` test code](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/#testing-with-the-run-function))

## Verification

The article and linked Go standard-library documentation were consulted as primary sources. Repository files were inspected read-only for the applicability section. Per the documentation-only scope, no Go tests, vet, or build checks were run.
