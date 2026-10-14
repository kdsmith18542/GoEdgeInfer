# Security Policy

## Secrets

- **No secrets in the repo. Ever.** No API keys, private keys, TLS keys,
  wallet seeds, or tokens in code, configs, or history.
- `configs/` must never contain `*.key` or `*.pem` private material. CI
  fails the build if a private key is added (see `plan.md` A3).
- Runtime secrets come from environment variables or a vault, documented in
  `configs/config.yaml` comments (key names only, never values).
- Note: test TLS keys were committed in 2026-05 and have been removed and
  rotated. If you find any secret in history, rotate it and report it.

## Auth model

- Inference + model endpoints: `X-API-Key` header.
- `/api/v1/mgmt/*`: JWT (when enabled) with `role`/`scope` claims → RBAC.
- Rate limiting is per-key (global default 10 rps / burst 20 until C3).

## Model integrity

Models are SHA256-verified at load; ECDSA signature verification is
available (`signature_verification` in config) and recommended for any
model fetched from a remote registry.

## Reporting

Security issues: open a private security advisory on GitHub (not a public
issue). Do not include secrets or exploit details in public issues.
