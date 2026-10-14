# Architecture

## Component map (as built)

```
cmd/                        server entrypoint; goedgeinferctl CLI
internal/
  app/          server wiring + lifecycle (builds router, pool, engines)
  api/          gin handlers + router — the HTTP contract (source of truth)
  grpcapi/      GoEdgeInferService server
  inference/    Engine interface; onnxruntime, tflite cgo/mock engines
  model/        ModelManager: registry, versioning, SHA256/ECDSA, S3 download
  worker/       goroutine pool, dispatch
  persistence/  BadgerDB durable queue (opt-in, see below)
  processing/   config-defined pre/post pipeline steps
  middleware/   X-API-Key → JWT → RBAC → per-key rate limit → request id
  security/     model signature verification
  config/       viper: config.yaml + env overrides
pkg/logging     zap-based structured logging
pkg/metrics     Prometheus collectors
proto/          infer.proto
```

## Request path (realtime mode)

```
HTTP/gRPC → middleware chain → validation → worker pool ─┐
                                                        ├─▶ engine session pool
                                                        │   (one session+tensor
                                                        │    set checked out
                                                        │    per request)
processing pipeline (pre) ──────────────────────────────┘
        │ inference
processing pipeline (post)
        │ → response (+ Prometheus/OTel recording)
```

## Concurrency rule

Engines own a **pool of session+tensor sets per loaded model** (size =
`onnxruntime.session_pool_size`, default = worker pool size). `Predict` checks one out exclusively and
returns it. No tensor is ever written by two goroutines. Rationale: the
2026-05 code shared one tensor set per model and released the lock before
inference — a data race that silently corrupted concurrent results
(plan.md A1). Regression test: 32 goroutines × 200 requests, `-race`,
output correctness asserted.

## Engines

`internal/inference.Engine` — LoadModel / GetModel / UnloadModel / Predict /
BatchPredict / GetModelInfo / ListModels, all version-aware. `BaseEngine`
provides registry bookkeeping; concrete engines implement execution.

| Engine | File | Status |
|---|---|---|
| `ONNXRuntimeEngine` | `onnx_runtime.go` | production; CPU + CUDA EP; lib path via `ORT_LIB_PATH`/config |
| `TFLiteEngine` (cgo) | `tflite_engine_cgo.go` | production; needs TF Lite C lib at build |
| `TFLiteMockEngine` | `tflite_engine_mock.go` | tests only — never in serving path |
| `GGUFEngine` (llama.cpp) | — | planned (plan.md B1); the LLM-at-edge path |

Dtypes: float32/float64, int8–int64, uint8–uint64, bool (plan.md A4).
Inputs: bare slice for single-input models, or a map keyed by input name
for multi-input models (plan.md A6).

## Queue modes

- **realtime** (default): requests dispatch through the in-memory worker
  pool; failures surface to the caller immediately.
- **durable**: requests are written to the BadgerDB queue before dispatch
  and recovered on startup. For offline/batch workloads and flaky networks —
  not the hot path. (`queue.mode` in config.)

The old design persisted every inference request to disk unconditionally;
that was a latency tax on traffic that didn't need it.

## Model registry

`ModelManager` (`internal/model`): `id:version` keys, local path or
`s3://bucket/key` download, SHA256 recorded at load, optional ECDSA
signature verification (`<path>.sig` + PEM public key). Hugging Face pulls
planned (B4).

## Auth model

- Inference + model endpoints: `X-API-Key`.
- `/api/v1/mgmt/*`: JWT (when enabled) with `role`/`scope` claims → RBAC
  (`admin`, `ops`); per-key rate limits and quotas (quotas planned, C3).
- Secrets come from env/vault. Never the repo (see `SECURITY.md`).
