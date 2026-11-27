# Serve an HTML-first web interface from a loosely coupled modular monolith

- **Status:** Accepted
- **Date:** 2026-11-27

## Context

Subman previously exposed only a JSON API. It needs a usable personal web interface without introducing a JavaScript framework, a frontend build pipeline, or a prematurely separate deployment. The interface must support the complete existing API workflow while keeping a future frontend/backend extraction practical.

The backend has no authentication or current-user concept. It exposes User selection and creation, Profile composition, Subscription and Card creation, Price Intervals, Cancellation, and Card Removal. The UI must represent those capabilities honestly rather than simulate login, editing, deletion, payment processing, or vendor integration.

The main architectural choice is the rendering and module boundary. A browser-rendered interface would consume the JSON API directly but would require substantial JavaScript for routing, rendering, forms, and state. Loopback HTTP from server-rendered handlers would preserve the JSON transport boundary inside one process but add operational and error-handling overhead without an immediate deployment benefit. Direct use of the concrete application service would be simple but would obscure the boundary needed for later extraction. These choices shape future UI work and are costly to reverse after templates, routes, and interactions accumulate.

The design decisions in this ADR were reviewed and accepted through an explicit design interview before implementation.

## Requirements

- Use HTML and CSS wherever practical, Go `html/template`, semantic server-rendered pages, ordinary forms, and Post/Redirect/Get.
- Keep every core workflow usable without JavaScript. Do not add a JavaScript or CSS framework, package manager, client-side router, or frontend build step.
- Keep the UI self-contained and loosely coupled from JSON API handlers and persistence, while running in the same process as the backend.
- Preserve every existing JSON API route and contract.
- Support User selection and creation, Profile, Subscription creation and details, Price Intervals, Cancellation, Card creation, and Card Removal.
- Use Profile as the central view, with separate detail, form, and confirmation pages. Cancellation and Removal require deliberate confirmation and explanatory copy.
- Provide responsive, keyboard-accessible pages with semantic controls, visible focus, accessible errors, and reduced-motion support.
- Provide one visual design with light and dark color schemes, initially following the operating system and remembering an explicit browser choice.
- Keep timestamps deterministic and unambiguous by entering and displaying them in UTC; continue using whole Swedish kronor.
- Reuse domain pricing rules rather than reimplementing effective Price Interval selection in presentation code.
- Keep local development infrastructure reproducible with a MongoDB Compose service while running the Go application on the host.
- Test the UI through its real HTTP surface using established memory repositories and an injectable clock.

## Considered Options

### Server-rendered modular monolith with a narrow application boundary

Place routes, handlers, view models, embedded templates, static assets, and tests in `internal/web`. Give the module a narrow application-service interface implemented by `app.Service`, and compose web and API handlers only in `main`.

This option is accepted because it minimizes browser JavaScript, preserves no-JavaScript workflows, and establishes a visible extraction seam without loopback HTTP or a second process.

### Static HTML and vanilla JavaScript over the JSON API

Serve static pages that fetch JSON and render the interface in the browser.

This option is rejected because routing, form state, rendering, error handling, and derived view models would require a substantial homemade client-side architecture. It would work against the HTML-first requirement and make core workflows unnecessarily dependent on JavaScript.

### Server-rendered handlers calling the JSON API over loopback HTTP

Keep the web UI server-rendered but make every application operation an HTTP request to the co-located JSON API.

This option is rejected for now because self-calls add configuration, network failure modes, and latency within one process. The narrow service interface provides a simpler seam that can later gain an HTTP-client implementation.

### Separate framework-based frontend application

Build and deploy an independent single-page application using a JavaScript framework.

This option is rejected because it violates the requested technology constraint, adds build and deployment complexity, and prematurely splits a personal application with a small API surface.

## Decision

Adopt a server-rendered, HTML-first modular monolith.

### Module and process boundary

- `internal/web` owns its application-service interface, route registration, handlers, view models, embedded templates, static CSS and JavaScript, and HTTP tests.
- The interface uses application use cases and domain queries, not API handlers or repositories. A future extraction can replace the service implementation with an HTTP client rather than rewrite handler behavior.
- `main.go` only composes the web and API handlers. Existing API paths under `/users` and `/healthz` remain unchanged. Web pages use `/profiles`, avoiding transport-route collisions.
- Templates and static assets are embedded in the Go binary. No separate asset pipeline or generated frontend artifacts are required.

### Interaction and presentation

- Use ordinary HTML forms and Post/Redirect/Get for mutations. Dedicated pages handle creation, details, Price Intervals, Cancellation, and Removal.
- User selection is explicit and is not presented as authentication. No sessions, passwords, or fake login screens are introduced.
- JavaScript is limited to the color-scheme switcher. It follows `prefers-color-scheme` initially and stores an explicit light/dark choice in browser storage; CSS custom properties provide one coherent design in both schemes.
- The interface is mobile-first, responsive, and accessible. Cancellation and Removal use confirmation pages rather than immediate actions or JavaScript confirmation dialogs.
- All date/time fields use clearly labeled UTC `datetime-local` values. Dates are displayed in UTC and amounts are formatted as whole SEK.

### Profile calculations

- Profile shows estimated Total Paid, phase counts, and separate monthly and annual recurring totals. It does not invent a blended monthly equivalent or next-charge forecast.
- Active Subscriptions contribute to recurring totals. Trialing Subscriptions are counted separately and Cancelled Subscriptions do not contribute; Total Paid remains aggregated across the retained history.
- Add `Subscription.CurrentAmountAt` as the reusable domain query for effective Price Interval selection. Presentation code must not duplicate the private interval-selection rule.

### Development infrastructure and verification

- Provide `compose.yaml` with the official MongoDB image, a named persistent volume, health check, and host port `27017`, matching the application's existing default URI. Do not containerize the Go application in this development compose file.
- Exercise web workflows through `web.NewServer`, real memory repositories, and a fixed clock. Retain the existing opt-in real-MongoDB process integration test for deployment wiring.

## Consequences

- Core pages and forms work without JavaScript, and the application needs no Node.js installation or frontend build.
- The web module can evolve separately from JSON transport code, but it is not yet independently deployable. Extraction still requires an HTTP-backed service adapter and deployment configuration.
- The `/profiles` namespace deliberately distinguishes HTML routes from existing JSON routes.
- Server rendering adds Go handlers, view models, and templates rather than browser state-management code.
- The domain exposes one additional explicit-time query, making current-amount behavior reusable and testable outside the UI.
- Browser theme preference is local to each browser. Core functionality does not depend on storage or JavaScript being available.
- UTC avoids hidden timezone conversion but is less convenient for Users who expect local-time input; changing that later requires an explicit timezone design.
- The recurring summary avoids annual-to-month rounding and forecasting rules that the domain does not define.
- Ordinary tests remain self-contained. MongoDB-backed verification still requires the opt-in process test and a disposable database.
- Future changes to this rendering or module-boundary approach should be evaluated against this ADR; material reversals require a superseding decision.

## Implementation Plan

1. Record the accepted design in this ADR and add it to the ADR index in the same changeset as the implementation.
2. Add `Subscription.CurrentAmountAt` with boundary and effective-interval tests.
3. Implement the self-contained `internal/web` module, including:
   - the application-service interface and injectable clock;
   - routes and handlers for all supported workflows;
   - embedded base, Profile, detail, form, confirmation, and error templates;
   - deterministic view models, sorting, UTC formatting, SEK formatting, and Profile summaries;
   - responsive CSS, light/dark color schemes, and minimal theme JavaScript;
   - complete HTTP workflow, validation, static-asset, and error-path tests.
4. Compose API and web handlers in `main.go` without changing existing JSON API routes.
5. Add the local MongoDB Compose service and named volume.
6. Update `README.md` and `AGENTS.md` with the web architecture, run instructions, infrastructure, and verification commands. Leave `CONTEXT.md` unchanged because domain terminology and lifecycle rules do not change.

## Verification Plan

- Format changed Go files with `gofmt` and run `go test ./...`, `go vet ./...`, and `go build ./...` from the repository root.
- Validate infrastructure with `docker compose config`.
- Confirm web tests cover User creation, Profile rendering, Card creation and Removal, Subscription creation and details, Price Intervals, Cancellation, summary rules, static assets, invalid IDs, and not-found pages.
- Confirm existing API tests still pass and the original JSON routes remain mounted.
- Confirm the UI contains no framework, package-manager, client-routing, or generated-build dependency, and core mutations use ordinary HTML forms.
- Run `git diff --check` and inspect the final diff for unrelated edits and generated artifacts.
- When a disposable MongoDB is available, optionally run `SUBMAN_TEST_MONGODB_URI=<test-only-uri> go test .`; otherwise report that process-level integration verification was skipped.
