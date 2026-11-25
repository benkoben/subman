# All amounts are Swedish kronor (SEK)

- **Status:** Accepted
- **Date:** 2026-10-29

Every price and billing amount in subman is stored and computed in SEK only, hard-coded at every construction site, even though go-money supports arbitrary currencies. This is deliberate scope reduction for a personal Swedish tracker: multi-currency would add per-amount currency handling, conversion questions, and display formatting for users who bill in one currency. Reversing this after real data accumulates means migrating stored amounts and touching every `money.New(..., price, SEK)` call — recorded here so nobody reads the hard-coding as an accident.

Considered and rejected: carrying currency per amount via go-money's currency field, deferring the decision until a second currency is actually needed.
