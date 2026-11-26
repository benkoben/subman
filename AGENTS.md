# AGENTS.md

## Project

Subman is a personal, vendor-agnostic subscription tracker written in Go. It records subscriptions, plans, prices, and payment-method metadata to organize and estimate recurring costs; it never processes payments.

- Use [CONTEXT.md](CONTEXT.md) as the canonical domain glossary. Its terminology and lifecycle rules take precedence over conflicting older documentation.
- Update `CONTEXT.md` when terminology or domain rules change. Update `README.md` when documented commands, capabilities, or architecture change.
- Keep changes minimal and scoped. Inspect relevant code and tests before editing, follow established patterns, and ask about consequential ambiguity.

## Repository map

- `internal/plan`: Plan value object and billing frequency.
- `internal/subscription`: Subscription aggregate, lifecycle, price intervals, and charge estimates.
- `internal/payment`: PaymentMethod behavior and Card metadata.
- `internal/user`: identity-only User records.
- `internal/repository`: generic repository contract with `memory` and `mongodb` adapters.
- `internal/app`: `Service` use cases composed over the repositories; the only place that coordinates aggregates and enforces ownership.
- `internal/api`: JSON HTTP API (`NewHandler`) over the `Service`, with an injectable clock for derived state.
- `main.go`: runner (`package main`) that wires the MongoDB repositories into the `Service`, serves the API, and shuts down gracefully. Configured by `MONGODB_URI`, `MONGODB_DATABASE`, and `HTTP_ADDR`.
- `docs/adr`: architectural decisions and their index.

Domain packages own their BSON mappings. Constructors validate invariants; lifecycle state is derived rather than stored. Follow existing functional-option and explicit-time `At` patterns where applicable. Keep `main.go` as composition only; put behavior in `internal/app` or `internal/api`.

## Decisions and plans

Routine implementation plans do not require ADRs. Create an ADR only when a decision meets **all three** criteria:

1. It is hard to reverse.
2. It is surprising without context.
3. It results from a real trade-off.

Write qualifying decisions as `Proposed` and obtain explicit user acceptance before implementing them. Use the format and lifecycle in [ADR 0002](docs/adr/0002-record-architectural-decisions.md), and update the [ADR index](docs/adr/README.md). Preserve accepted ADRs as history; material changes require a superseding ADR. Deliver an ADR in the same changeset as its implementation.

## Branch and pull request naming

- Always name branches `<type>/<short-description>`, using lowercase kebab-case for the description (for example, `feat/subscription-reminders`).
- Always title pull requests `(<type>) <short description>` (for example, `(feat) Add subscription reminders`).

## Determinism and tooling

- Strive for deterministic, repeatable work using the tools, systems, and patterns already in place. Do not introduce replacement or parallel tooling without justification.
- Use established testing seams, including explicit-time variants, and control other sources of nondeterminism where practical.
- Never commit generated executables, libraries, test binaries, archives, or other build artifacts. Add their paths or patterns to `.gitignore` in the same changeset that introduces the generating workflow.
- Intentional binary assets such as images, fonts, or fixtures require explicit user approval. Prefer text-based formats where practical.
- Obtain explicit authorization before destructive operations or Git mutations, including committing, pushing, rebasing, or rewriting history.

## Verification

Format changed Go files with `gofmt`, then run the applicable checks from the repository root:

```sh
go test ./...
go vet ./...
go build ./...
```

Do not claim unverified success. Distinguish pre-existing failures from regressions and report both the checks run and their results. For documentation-only changes, Go checks are unnecessary; validate the changed documentation and say which checks were skipped.
