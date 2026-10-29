# Subman — subscription tracking

Subman is a personal, vendor-agnostic tracker: users record the subscriptions they pay for so they can organize them and see what they cost. Subman never processes payments — it only records and estimates.

## Records

**Subscription**:
A user's record of an ongoing paid relationship to a plan. Born Trialing or Active; ends Cancelled, which is terminal — resubscribing creates a new Subscription. Its current phase is derived from its dates (startDate, trialEnd, cancelledAt), never stored.
_Avoid_: registration, sign-up

**Trial**:
The free opening phase of a Subscription. Entered as a length; the trial end date is derived from the start. Once it passes, the Subscription is Active and charges begin.
_Avoid_: trail (typo), free trail, "a trial that converts into a subscription" (it is a phase of one, not a separate record)

**Plan**:
The user-entered description of what is subscribed to: name, vendor, description, billing frequency. A value object owned by its Subscription — two users' "Netflix Premium" are unrelated descriptions.
_Avoid_: offer (implies a shared catalog), product

**User**:
The person whose subscriptions and payment methods subman tracks. Identity only — name and email; owns a Wallet.
_Avoid_: account

**Profile**:
The view of a User's Subscriptions and Wallet — not a stored record.
_Avoid_: dashboard, member page

## Pricing

**Price Interval**:
An entry in a Subscription's pricing history: a billing amount effective from a given date. The ordered list of intervals replaces a single flat price.
_Avoid_: price change, rate

**Intro Offer**:
The first paid Price Interval, at a reduced amount for a limited time, before the regular amount applies.
_Avoid_: Reduction (old term), discount, deal

**Billing Frequency**:
How often a plan charges: monthly or annually.

**Amount**:
A billing figure, always Swedish kronor — see ADR 0001.
_Avoid_: price, cost (both too loose to pin down which figure is meant)

**Total Paid**:
The sum of charges a Subscription has incurred from its first charge until now or its cancellation, counted in started billing periods — vendors bill upfront. An estimate reconstructed from the price timeline; subman never observes real payments.
_Avoid_: spend, cost so far

## Payment

**PaymentMethod**:
A user's record of how a Subscription is paid. Optional metadata; its purpose is bookkeeping, never payment processing.
_Avoid_: Means, payment means

**Wallet**:
The set of PaymentMethods a User owns. Subscriptions reference wallet entries by ID, never embed them.
_Avoid_: payment profile

**Card**:
A PaymentMethod of debit or credit type with a month-granular expiry date, so affected Subscriptions can be surfaced when it expires.
_Avoid_: debid (typo)

**Card Expiry**:
The month a Card ceases to be valid. Derived from the expiry date, never a stored status.
_Avoid_: expired flag, status flip

**Removal**:
A user's timestamped statement (removedAt) that a PaymentMethod is retired. The wallet entry and every Subscription reference survive — it is not a deletion.
_Avoid_: cancel, cancellation (reserved for Subscriptions), delete

## Lifecycle

**Cancellation**:
The user's in-app statement that a Subscription is no longer active spend, recorded with a timestamp. Cancelling at the vendor is a real-world act subman cannot observe and does not model.
_Avoid_: vendor cancellation, pausing
