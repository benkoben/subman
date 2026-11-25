# Record only architecturally significant decisions as ADRs

- **Status:** Accepted
- **Date:** 2026-11-25

## Context

Subman needs repository guidance that helps coding agents implement changes safely and consistently. The previous `AGENTS.md` contained unrelated course-specific teaching restrictions and was removed. The replacement must describe this repository without duplicating detailed documentation that could become stale.

The project also needs durable records of consequential design choices. An initial proposal considered recording every non-trivial implementation plan as an ADR. That would preserve implementation reasoning, but it would also mix ordinary task planning with architectural history and create documentation overhead for changes that are easy to reverse or involve no meaningful trade-off. Traditional ADR practice provides a more focused threshold.

## Requirements

- Treat agents as implementation partners that inspect, plan, implement, verify, and review changes.
- Keep the root `AGENTS.md` concise and repository-wide; use nested files only for genuinely specialized instructions.
- Use ADRs for architecturally significant decisions rather than for every implementation plan.
- Require an ADR only when a decision is hard to reverse, surprising without context, and the result of a real trade-off.
- Require explicit user acceptance of a proposed ADR before implementing its decision, unless the user explicitly authorizes otherwise.
- Preserve proposed, rejected, accepted, and superseded decisions as durable project history.
- Give ADRs a consistent location, naming scheme, template, lifecycle, and index.
- Keep domain terminology in `CONTEXT.md` authoritative and plan related documentation updates with the implementation.
- Require applicable formatting and verification, honest reporting, minimal scoped changes, and explicit authorization for destructive operations and Git mutations.
- Strive for deterministic, repeatable practices using the tools, systems, and patterns already established in the repository.
- Add generated build and test binaries to `.gitignore`; permit intentional binary assets only with explicit user approval.

## Considered Options

### Record only architecturally significant decisions

This follows traditional ADR practice and limits documentation to decisions whose rationale has lasting value. Routine implementation plans can remain in conversations or task trackers unless they uncover a qualifying architectural decision. This option is accepted because it preserves important context without turning ADRs into a general task-management system.

### Keep all plans and decisions in conversations or an external task tracker

This avoids additional repository files. It was rejected because the rationale for consequential architectural decisions would be separated from the code and could become unavailable to future contributors and agents.

### Record every action, including mechanical edits

This is superficially simple but was rejected because ADRs for actions such as a requested file deletion or typo correction would add noise without preserving meaningful decisions.

### Record every non-trivial implementation plan as an ADR

This would preserve reasoning behind all multi-step or design-affecting work. It was rejected because ordinary implementation plans are not necessarily architectural decisions: many are easy to reverse, unsurprising, or involve no real trade-off. Including them would obscure the architectural record and create disproportionate documentation and approval overhead.

## Decision

Follow traditional ADR practice. An ADR is required only when all three of the following are true:

1. **Hard to reverse:** changing the decision later would have a meaningful cost.
2. **Surprising without context:** a future reader would need the rationale to understand why the code or architecture works this way.
3. **A real trade-off:** genuine alternatives existed and the chosen approach has consequences worth recording.

Typical qualifying decisions include architectural shape, integration patterns, technologies with meaningful lock-in, ownership and boundaries, deliberate deviations from common practice, and constraints not visible in the code. Routine features, bug fixes, refactors, test plans, library choices with little lock-in, and mechanical actions do not automatically require ADRs.

Implementation plans do not require ADRs merely because they contain choices, sequencing, trade-offs, or multiple steps. If planning or implementation uncovers a decision that meets all three criteria, record that decision before implementing it.

### Decision granularity and approval

Each coherent architectural decision has one ADR covering its related rationale and consequences. Independently meaningful decisions use separate ADRs. Exploratory work needs an ADR only once it produces a concrete qualifying decision.

ADRs begin with status `Proposed`. Explicit user acceptance changes the status to `Accepted` before the decision is implemented. Rejected or abandoned proposals remain in the repository as `Rejected`, with a short rationale.

### ADR storage and format

ADRs live in `docs/adr/` and use sequential, zero-padded filenames such as `0002-record-architectural-decisions.md`. Each ADR contains:

- title, status, and date;
- context;
- requirements;
- considered options;
- decision;
- consequences;
- implementation plan; and
- verification plan.

Sections may be short but must not be silently omitted. Use `None` when a section has no content. The implementation and verification sections cover work needed to carry out the architectural decision, not an unrelated feature's entire task list. `docs/adr/README.md` provides a chronological index with links and statuses.

Accepted ADRs are historical records. Correct broken links and factual errors without rewriting their decisions. A material change to the decision requires a new ADR that supersedes the earlier one and must be accepted before work proceeds along the changed approach. The applicable ADR is delivered in the same changeset as its implementation.

### Repository guidance

The root `AGENTS.md` links to detailed documentation rather than duplicating it. It identifies `CONTEXT.md` as the canonical domain-language reference. Terminology or domain-rule changes update `CONTEXT.md`; changes to documented commands, capabilities, or architecture update the README. Each ADR's implementation plan identifies the necessary documentation updates.

The guidance requires formatting changed Go files and running applicable `go test ./...`, `go vet ./...`, and `go build ./...` checks. Agents report pre-existing failures separately from regressions and never claim unverified success. No application run command is documented until a runnable application exists.

Changes remain minimal and scoped. Consequential ambiguity requires clarification. Destructive operations and Git mutations such as committing, pushing, rebasing, or rewriting history require explicit authorization.

Strive for deterministic, repeatable practices using the tools, systems, and patterns already established in the repository. Avoid introducing replacement or parallel tooling without justification. Keep tests reproducible by using established seams, such as the existing explicit-time variants, and by controlling other sources of nondeterminism where practical.

Compiled executables, libraries, test binaries, archives, and other generated binary artifacts must not be committed and must be covered by `.gitignore`. When a build, test, or development workflow introduces a new generated output, add an appropriate ignore rule in the same changeset. Intentional binary assets such as images, fonts, or binary fixtures require explicit user approval; text-based formats are preferred where practical.

## Consequences

- The reasoning behind consequential architectural decisions remains available beside the code, including rejected and abandoned approaches.
- Routine implementation work does not incur ADR documentation or approval overhead.
- Reasoning for non-architectural implementation plans may remain outside the repository unless another durable medium is chosen.
- Contributors and agents must apply the three-part threshold deliberately and ask for clarification when its application is ambiguous.
- Agents and contributors must pause for acceptance before implementing a proposed architectural decision.
- Material changes to accepted decisions require a superseding ADR and another approval cycle.
- The ADR index and statuses require maintenance whenever decisions change.
- Existing tools and test seams remain the default, so introducing new tooling requires justification and may itself require an ADR when it meets the decision threshold.
- `.gitignore` must evolve with build and test workflows so generated binaries remain excluded by convention rather than by vigilance alone.
- Intentional binary assets need deliberate review and explicit approval; text-based alternatives may not always be available.
- The root agent guidance stays enforceable and readable because detailed rationale remains in ADRs.

## Implementation Plan

1. Obtain explicit user acceptance of this ADR, then change its status to `Accepted`.
2. Create a concise root `AGENTS.md` containing the operating rules, project pointers, architecture summary, verification commands, deterministic-workflow expectations, and traditional ADR threshold, with a link to this ADR for the full rationale.
3. Add `docs/adr/README.md` indexing ADR 0001 and this ADR with their statuses.
4. Add historically accurate status and date metadata to ADR 0001 without changing its original decision text.
5. Review `.gitignore` against the outputs of documented build and test workflows, adding specific rules for any uncovered generated binaries.
6. Review all guidance and links for consistency with the accepted decision.

## Verification Plan

- Confirm that every planned ADR section is present and that the index reflects current statuses.
- Confirm that `AGENTS.md` links to `CONTEXT.md`, this ADR, and the ADR index without duplicating their detailed content.
- Confirm that the guidance does not require ADRs for routine implementation plans and applies all three parts of the architectural-significance threshold.
- Confirm that deterministic practices and existing tools and systems are the default, including established explicit-time testing seams.
- Confirm that `.gitignore` covers the generated binary outputs of documented workflows and that the guidance requires future outputs to be ignored in the same changeset.
- Confirm that no nonexistent application run command or unsupported tooling is documented.
- Run `git diff --check` and inspect the final diff for unintended files, generated artifacts, or binaries.
- Run Go formatting, tests, vetting, and builds only if the implementation changes Go source; otherwise report the documentation-only verification performed.
