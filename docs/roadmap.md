# Roadmap: done / now / next

> Replaces the future-tense Phase 1–4 in the old blueprint. Updated 2026-10-13.
> Execution detail lives in `plan.md`.

## Done

- ONNX Runtime engine (load/predict/batch, CPU + CUDA provider)
- TFLite engines (cgo + mock)
- Versioned model manager: SHA256 integrity, ECDSA signatures, S3/MinIO
- HTTP API (`/api/v1`) + gRPC (`Infer`, `StreamInfer`)
- Worker pool + (formerly always-on) BadgerDB queue
- API-key / JWT / RBAC auth, rate limiting, request IDs
- Prometheus metrics, OpenTelemetry tracing, structured logging
- `goedgeinferctl` CLI, Dockerfile, config system

## Now (plan.md Phase A — correctness)

Session-pool race fix · configurable ORT path · secrets purge · quantized
dtypes · honest batching · multi-input · CLI/server contract fix · doc sync.
**Gate: no Phase B until A1–A3 are merged.**

## Next (plan.md Phase B — market relevance)

GGUF/llama.cpp backend · OpenAI-compatible API (`/v1/chat/completions`,
`/v1/embeddings`) · real dynamic batching · Hugging Face pulls · published
benchmarks.

**B-gate (kill criteria):** quantized 1–3B GGUF at ≥10 tok/s on a $6–12/mo
CPU VPS, and the OpenAI API working against 3+ real clients. Fail either →
stop, narrow, or shelve. Do not start Phase C on vibes.

## Later (plan.md Phase C — revenue)

Per-key metering → plan quotas → Stripe + BTCPay billing → hosted beta.

**C-gate:** ≥5 paying/committed users 8 weeks after beta opens, or
distribution (not code) is the problem.

## Explicitly not on the roadmap

Training · GPU-cluster orchestration · K8s operators · XAI / RL agents /
digital twins / federated learning (cut from the old blueprint) · rebuilding
inference kernels · frameworks beyond ONNX/TFLite/GGUF.
