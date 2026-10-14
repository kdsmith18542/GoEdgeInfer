# GoEdgeInfer: Architecture Blueprint (revised 2026-10)

NO STUBB!! PRODUCTION GRADE CODE ONLY!! SPECS ARE IN THE DOCS FOLDER!!

> This is the revised blueprint. The 2026-05 original described a generic
> "edge AI agent" in future tense; most of it is now built, and the product
> direction has pivoted (see §1). Sections marked **[PIVOT]** changed with the
> pivot; **[DONE]** already exists in code; **[NEXT]** is plan.md Phase B/C.

## 1. Product thesis [PIVOT]

GoEdgeInfer is a **metered, multi-tenant inference server for quantized small
models on CPU-only edge hardware**. The product is the SaaS-ification —
metering, tenancy, billing, and an OpenAI-compatible API — not the inference
kernels. llama.cpp and ONNX Runtime already exist and are not reimplemented.

The wedge: a quantized 1–3B model serving at ≥10 tok/s on a $6–12/mo CPU VPS.
If the B-gate in plan.md fails, the cost thesis fails with it.

Explicit non-goals: training, GPU-cluster orchestration, K8s operators,
explainable-AI/RL-agents/digital-twins/federated-learning (cut from the old
§7 — scope creep, not the product), rebuilding inference kernels.

## 2. Core principles

- **Correct under concurrency.** Every engine is safe for concurrent Predict;
  session/tensor pooling per model, `-race` clean in CI. (This was a real bug:
  plan.md A1.)
- **Quantized-first.** int8/uint8 dtypes are table stakes for edge, not a
  Phase-4 consideration. float32-only was a gap (plan.md A4).
- **Ecosystem-compatible.** OpenAI-compatible endpoints alongside the native
  API, so existing clients work unmodified (plan.md B2).
- **Single binary, no required services.** SQLite default metering, BadgerDB
  queue opt-in. Self-hosting must stay one binary + one config file.
- **No silent failures.** No mocks in the serving path; batching that doesn't
  batch gets renamed, not shipped (plan.md A5).
- **Docs in the same PR as code.** Doc drift is how the old README lied about
  "not implemented yet" endpoints for months.

## 3. Architecture [DONE, with corrections]

Single agent process. Actual component map (the old blueprint's
`internal/runtime`, `internal/manager`, `internal/health` never existed):

```
                    ┌──────────────────────────────────────┐
                    │           GoEdgeInfer                │
  clients ──HTTP──▶ │  api (gin) ──▶ worker pool ──┐       │
         ──gRPC──▶ │  grpcapi ────────────────────┘       │
                    │      │                               │
                    │      ▼                               │
                    │  processing pipeline                 │
                    │  (pre/post steps, config-defined)    │
                    │      │                               │
                    │      ▼                               │
                    │  inference engines (Engine iface)    │
                    │   ├── onnxruntime  [DONE]            │
                    │   ├── tflite (cgo) [DONE]            │
                    │   ├── tflite mock [tests only]      │
                    │   └── gguf/llama.cpp [NEXT, B1]     │
                    │      │                               │
                    │      ▼                               │
                    │  model manager                       │
                    │  (versioning, SHA256, ECDSA sig,     │
                    │   S3/MinIO + HF download)           │
                    │                                      │
                    │  persistence: BadgerDB queue         │
                    │   mode=realtime (default) | durable  │
                    │   [PIVOT — was always-on]           │
                    │                                      │
                    │  middleware: api-key → JWT → RBAC → │
                    │   per-key rate limit → request id   │
                    │                                      │
                    │  observability: Prometheus, OTel     │
                    │  mgmt: /api/v1/mgmt/* (JWT, RBAC)   │
                    │  billing: metering → Stripe/BTCPay  │
                    │   [NEXT, plan.md C1–C2]             │
                    └──────────────────────────────────────┘
```

### Concurrency model [PIVOT]

The old blueprint said "implement a pool of ONNX Runtime sessions" — that
line was never implemented and its absence caused the A1 data race. The rule
now: **each engine maintains a pool of N session+tensor sets per loaded
model** (N = worker pool size, configurable); `Predict` checks one out
exclusively and returns it. No shared mutable tensors across goroutines,
ever. Concurrency tests are mandatory per engine.

### Backends

| Backend | Status | Notes |
|---|---|---|
| ONNX Runtime (`onnxruntime_go`) | DONE | CPU + CUDA provider; lib path from config/env (`ORT_LIB_PATH`), never hardcoded (was a bug: A2) |
| TFLite cgo | DONE | needs TF Lite C library at build |
| TFLite mock | tests only | must never serve production traffic |
| GGUF / llama.cpp | NEXT (B1) | the LLM-at-edge path; pick bindings with prebuilt binaries |

### Queue modes [PIVOT]

- `realtime` (default): in-memory channel dispatch, client-visible errors,
  no disk writes on the hot path.
- `durable`: BadgerDB store-and-forward for offline/batch workloads;
  recovery on startup. Opt-in via `queue.mode`.

The old spec made durability the default for all inference traffic — wrong
for real-time edge; disk on the hot path is a latency tax most requests
shouldn't pay.

## 4. APIs

Native (DONE, see `docs/API.md` for the full table):
`GET/POST /api/v1/models…`, `POST /api/v1/infer/:model_id[/versions/:v]`,
`POST /api/v1/infer/batch/…`, `/api/v1/mgmt/*` (JWT), gRPC
`GoEdgeInferService.{Infer,StreamInfer}`.

OpenAI-compatible [NEXT] (B2): `POST /v1/chat/completions` (SSE + non-stream),
`POST /v1/embeddings`, `GET /v1/models`. Acceptance: OpenAI Python SDK works
with only `base_url` changed.

## 5. Model management [DONE]

Versioned load/unload/hot-reload via API, CLI, or config; S3/MinIO + (planned)
Hugging Face pulls with SHA256 verification; ECDSA signature verification
optional at load; model IDs/versions must not contain `:` (key separator).

## 6. Billing [NEXT] (plan.md C1–C2, see `docs/billing.md`)

Per-API-key metering (tokens in/out, requests, compute-ms; SQLite default) →
plan quotas → Stripe metered subscriptions + self-hosted BTCPay Server
(Bitcoin/Lightning, 0% fees, priced in USD). Crypto is a checkout option,
not the only rail.

## 7. Security

API keys on inference paths; JWT + RBAC on `/api/v1/mgmt/*`; per-key rate
limits; model signature verification; **no secrets in the repo**
(`configs/*.key` committed in 2026-05 — removed, rotated, CI-grepped; see
`SECURITY.md`, plan.md A3).

## 8. Roadmap

Superseded by `docs/roadmap.md` (done / now / next) and `plan.md` (phases +
kill gates). The old Phase 1–4 text below is kept for history and marked.

<details>
<summary>Original 2026-05 roadmap (historical — mostly built)</summary>

- Phase 1 (MVP): ONNX load+infer, HTTP API, config, logging, tests — DONE.
- Phase 2: gRPC, pre/post-processing, Prometheus, persistent buffer — DONE
  (buffer since pivoted to opt-in).
- Phase 3: hot-reload, S3 registry, OTel, CLI, remote config — DONE.
- Phase 4: execution providers, signature verification, quantization — DONE
  except quantization dtype support (moved to Phase A).
</details>

## 9. Benchmarks

`docs/benchmarks.md` (new, B6): tokens/sec, TTFT, peak RSS for reference
models on reference hardware, re-run per release. This file is marketing —
treat it accordingly.
