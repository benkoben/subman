# Subman — subscription tracking

Subman is a personal, vendor-agnostic tracker: users record the Subscriptions they pay for so they can organize them and estimate what they cost. Subman never processes payments; it only records and estimates. The domain terminology is defined in [CONTEXT.md](CONTEXT.md), which takes precedence over anything else in this document.

## Requirements

- Go 1.26.5 or newer (see `go.mod`)
- MongoDB reachable at `MONGODB_URI`

## Running

```sh
go run .
```

The server connects to MongoDB, verifies the connection with a bounded startup timeout, and serves the HTTP API. `SIGINT` and `SIGTERM` trigger a graceful shutdown that stops accepting requests, waits up to 10 seconds for in-flight requests, and disconnects from MongoDB.

| Variable           | Default                     | Purpose                              |
| ------------------ | --------------------------- | ------------------------------------ |
| `MONGODB_URI`      | `mongodb://localhost:27017` | MongoDB connection string            |
| `MONGODB_DATABASE` | `subman`                    | Database holding the collections below |
| `HTTP_ADDR`        | `:8080`                     | Address the HTTP server listens on   |

Records are stored in the collections `users`, `cards`, and `subscriptions`. There are no migrations; collections are created on first write.

## Verification

```sh
go test ./...
go vet ./...
go build ./...
```

The process-level integration test uses a real MongoDB deployment and is skipped by default. To run it against a disposable deployment, set `SUBMAN_TEST_MONGODB_URI`; the credentials must be allowed to create, write, and drop the test's unique database:

```sh
SUBMAN_TEST_MONGODB_URI=mongodb://localhost:27017 go test .
```

## Architecture

- `internal/plan`: Plan value object and billing frequency.
- `internal/subscription`: Subscription aggregate, lifecycle, Price Intervals, and Total Paid.
- `internal/payment`: PaymentMethod behavior and Card metadata.
- `internal/user`: identity-only User records.
- `internal/repository`: generic repository contract with `memory` and `mongodb` adapters.
- `internal/app`: use cases over the repositories (the Service).
- `internal/api`: JSON HTTP API over the Service. `NewServer` composes the `http.Handler`, `routes.go` maps the complete API surface, and resource handlers are maker functions with explicit dependencies.
- `main.go`: the minimal process entry point and injectable `run` function. `run` wires the MongoDB repositories into the Service and serves the API using explicit context, environment, error-output, and listener dependencies.

## HTTP API

All bodies are JSON. Identifiers in paths are UUIDs. Errors are returned as `{"error": "..."}`:

- `400` for malformed IDs or JSON, unknown or trailing JSON values, missing required fields, and rejected domain input.
- `404` for unknown resources, and for resources owned by another User (they are not revealed).
- `500` for internal failures, with a generic message.

Conventions:

- Timestamps are RFC 3339 (for example `2026-11-25T09:00:00Z`).
- Amounts are integer Swedish kronor (SEK), passed to go-money. See [ADR 0001](docs/adr/0001-all-amounts-in-sek.md).
- Derived state (Subscription phase, Card expiry and activity, Total Paid) is evaluated at server time when the response is produced. It is never stored or supplied by the client.
- Request bodies are decoded strictly: unknown fields and anything after the single JSON value are rejected.
- List responses are arrays in an unspecified order.
- Removal and Cancellation keep their records and references. Both are idempotent: the earliest timestamp wins.

### Health

- `GET /healthz` — returns `200` with `{"status":"ok"}` once the HTTP server is running. MongoDB connectivity is verified before the server starts.

### Users

- `POST /users` — body `{"name", "email"}`. Returns `201` with the User `{"id", "name", "email"}`.
- `GET /users` — returns `200` with an array of Users.

### Cards (Wallet)

- `POST /users/{userID}/cards` — body `{"name", "cardType", "expiryYear", "expiryMonth"}`, where `cardType` is `debit` or `credit` and `expiryMonth` is 1–12. Returns `201` with the Card. Unknown User: `404`.
- `GET /users/{userID}/cards` — returns `200` with the User's Cards.
- `POST /users/{userID}/cards/{cardID}/removal` — body `{"removedAt"}`. Records a Removal and returns `200` with the Card, whose `removedAt` is set. The Card stays in the wallet and keeps its Subscription references.

A Card response contains `id`, `name`, `cardType`, `expiryYear`, `expiryMonth`, `removedAt`, `expired`, and `active`. `expired` and `active` are derived at server time.

### Subscriptions

- `POST /users/{userID}/subscriptions` — body:

  ```json
  {
    "plan": {
      "name": "Netflix Premium",
      "vendor": "Netflix",
      "description": "optional",
      "billingFrequency": "monthly"
    },
    "initialAmount": 179,
    "startDate": "2026-11-01T00:00:00Z",
    "trialDays": 30,
    "paymentMethodID": "optional card UUID"
  }
  ```

  `billingFrequency` is `monthly` or `annually`. `startDate` is required. `trialDays` is optional and must be positive; it creates a Trial. `paymentMethodID` is optional and must reference a Card in the same User's Wallet. Returns `201` with the Subscription.
- `GET /users/{userID}/subscriptions` — returns `200` with the User's Subscriptions.
- `POST /users/{userID}/subscriptions/{subscriptionID}/cancellation` — body `{"cancelledAt"}`, required. Records a Cancellation and returns `200` with the Subscription. A timestamp before the start date is `400`.
- `POST /users/{userID}/subscriptions/{subscriptionID}/price-intervals` — body `{"from", "amount"}`, both required. Adds a Price Interval to the pricing history and returns `200` with the Subscription. `amount` must be positive. A `from` before the first charge or a duplicate `from` is `400`.

A Subscription response contains:

- `id`, `plan` (`name`, `vendor`, `description`, `billingFrequency`), `paymentMethodID`
- `startDate`, `trialEnd`, `cancelledAt`
- `intervals`: the Price Intervals as `{"from", "amount"}`
- `phase`: `trialing`, `active`, or `cancelled`, derived from the dates at server time
- `totalPaid`: `{"amount", "currency"}`, an estimate of charges incurred from the first charge until now or cancellation, counted in started billing periods. It is reconstructed from the price timeline; Subman never observes real payments.

### Profile

- `GET /users/{userID}/profile` — returns `200` with `{"user", "cards", "subscriptions"}`, composed on demand from the User's Wallet and Subscriptions. It is not stored.

## Scope

Subman records, organizes, and estimates. It does not charge cards, contact vendors, or track vendor-side cancellations. A Cancellation in Subman is only an in-app statement; cancelling at the vendor is a separate act that Subman does not model.
