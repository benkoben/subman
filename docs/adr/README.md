# Architecture Decision Records

ADRs preserve the rationale for architecturally significant decisions. Use one when a decision is hard to reverse, surprising without context, and the result of a real trade-off; see [ADR 0002](0002-record-architectural-decisions.md) for the complete workflow and format.

Statuses are `Proposed`, `Accepted`, `Rejected`, and `Superseded`. A proposed ADR requires explicit user acceptance before its decision is implemented. Supersede accepted decisions rather than silently rewriting them.

| ADR | Status | Decision |
| --- | --- | --- |
| [0001](0001-all-amounts-in-sek.md) | Accepted | All amounts are Swedish kronor (SEK) |
| [0002](0002-record-architectural-decisions.md) | Accepted | Record only architecturally significant decisions as ADRs |
| [0003](0003-structure-http-service-around-explicit-composition.md) | Accepted | Structure the HTTP service around explicit composition |
| [0004](0004-serve-html-first-web-interface-from-modular-monolith.md) | Accepted | Serve an HTML-first web interface from a loosely coupled modular monolith |
