# Billing & Metering [PLANNED — plan.md Phase C]

## Metering (C1)

Per-API-key usage accounting, recorded on every inference:

- `tokens_in`, `tokens_out` (LLM/text engines), `requests`, `compute_ms`
- Stored in SQLite by default (single-binary story), Postgres optional
- `GET /v1/usage` per key; Prometheus token counters (see `docs/metrics.md`)
- Keys are hashed at rest; only `api_key_id` prefixes appear in metrics/logs

## Plans → quotas → enforcement (C2/C3)

```
plan (hobby/pro/scale) → quotas (tok/min, req/day, models) → middleware
```

Extend `internal/middleware/rate_limit.go` from global to per-key using the
entitlement service. Quota exhaustion returns `429` with
`X-Quota-Reset` headers, not silent drops.

## Payment rails

- **Stripe** — metered subscriptions; webhooks update entitlements.
- **BTCPay Server** — self-hosted, 0% processing fees, Bitcoin + Lightning.
  Price in USD, settle in BTC, convert on a schedule. Crypto is a checkout
  *option*, not the only rail — the audience (self-hoster devs) skews
  crypto-comfortable, but Stripe covers everyone else.

## Why this order

Metering ships before billing UI: you can't bill what you don't measure, and
usage data from the beta informs the actual plan tiers (don't invent pricing
before traffic exists).
