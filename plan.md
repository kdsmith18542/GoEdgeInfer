# GoEdgeInfer — Build Plan

Date: 2026-10-13. Author: Keith, with review by Muse.
Status: Phase A complete (2026-10-14). Phase B not started.

## North star

A metered, multi-tenant inference server for quantized small models on CPU-only
edge hardware — with crypto + fiat billing. The product is the *SaaS-ification*
(metering, tenancy, billing, OpenAI-compatible API), not the inference kernels;
llama.cpp and ONNX Runtime already exist and are not to be reimplemented.

Non-goals: training, GPU-cluster orchestration, K8s operators, new model formats
beyond ONNX / TFLite / GGUF, rebuilding inference kernels.

## Verified starting state (2026-10-13)

More built than the README claims (README/docs describe the old mock; treat all
docs as suspect until Phase A7):

- `internal/inference/onnx_runtime.go` — real ONNX Runtime engine: LoadModel /
  Predict / BatchPredict via `yalue/onnxruntime_go`. CUDA provider optional.
- `internal/inference/tflite_engine_cgo.go` + `tflite_engine_mock.go` — cgo and
  mock TFLite engines.
- `internal/inference/engine.go` — `Engine` interface + `BaseEngine` (whose
  Predict returns `ErrNotImplemented`) + `MockEngine`.
- `internal/model/model.go` — `ModelManager` with SHA256 integrity check and
  ECDSA signature verification on load; S3/MinIO download support; versioned
  model keys (`id:version`).
- `internal/worker/pool.go` + `internal/persistence/queue.go` — goroutine worker
  pool with BadgerDB persistent queue and startup recovery.
- `internal/api/` (gin), `internal/grpcapi/`, `internal/middleware/` (API key,
  JWT, RBAC, rate limit, request ID), `pkg/metrics` (Prometheus),
  OpenTelemetry tracing, `cmd/goedgeinferctl` CLI, `proto/infer.proto`.
- 7 commits, last 2026-05-20. Go 1.23. Tests exist per package.

---

## Phase A — Correctness & hygiene (target: 2–3 weeks solo)

Do not start Phase B until A1–A3 are done. These are correctness and safety, not
features.

### A1. Fix the Predict data race (P0) ✅ 2026-10-14

File: `internal/inference/onnx_runtime.go` (`Predict`, ~L341).

Bug: input/output tensors are allocated once at `LoadModel`; `Predict` takes
`RLock`, copies the model pointer, releases the lock, then writes request data
into the *shared* input tensor and runs the session. Concurrent requests
interleave tensor writes → silently corrupt results.

Fix: per-model session pool. At load, create N session+tensor sets per model
(N = config, default = worker pool size); `Predict` checks one out via channel,
uses it exclusively, checks it back in. Same treatment for the TFLite cgo
engine if it shares tensors.

Acceptance:
- New test hammers one loaded model from 32 goroutines × 200 requests with
  distinct inputs under `go test -race ./...` — zero races, all outputs correct.
- p99 latency regression < 15% vs single-flight baseline.

### A2. Configurable ONNX Runtime library path (P0) ✅ 2026-10-14

File: `internal/inference/onnx_runtime.go` (`NewONNXRuntimeEngine`, ~L40).

Bug: shared-library path hardcoded to
`$HOME/onnxruntime/onnxruntime-linux-x64-1.22.0/lib/libonnxruntime.so`.
Breaks on any other machine, container, or CI.

Fix: `config.yaml` `onnxruntime.lib_path` + env override `ORT_LIB_PATH`;
fallback search of well-known locations; clear error listing what was tried.
Dockerfile downloads the pinned ORT release at build time (pin version in one
place, e.g. `scripts/ort-version.txt`).

Acceptance: fresh `git clone` → `docker build` → `docker run` serves the test
model with no host-side ORT install.

### A3. Secrets hygiene (P0) ✅ 2026-10-14

`configs/` currently commits `server.key`, `server.crt`, `ca.crt` to a public
repo. They are in git history forever.

Fix: rotate anything that was ever real; delete from tree; gitignore
`*.key`, `*.pem` (keep `*.crt` only if test fixtures, clearly labeled);
document in a new `SECURITY.md` that secrets come from env/vault, never the
repo. If any committed key was ever used in production, treat it as
compromised.

Acceptance: `git log --all -- configs/*.key` shows no keys added after this
date; `SECURITY.md` exists; CI fails if a private key is added
(pre-commit hook or CI grep).

### A4. Quantized dtype support (P1) ✅ 2026-10-14

Tensors are hardcoded float32. For an *edge* server this is table stakes.

Fix: extend tensor creation/conversion to int8/uint8 (read dtype from
`ort.GetInputOutputInfo`); keep float32 default.

Acceptance: load a quantized (int8) ONNX model, predict, correct output.

### A5. Honest batching (P1) ✅ 2026-10-14 (rename path)

`BatchPredict` currently loops `Predict` sequentially — no throughput gain,
misleading name.

Fix: implement true batched inference through the ORT batch dimension where
the model supports it; otherwise rename to `PredictSequential` and document
the limitation. Do not ship a lie in a public API.

**Shipped (2026-10-14):** rename path. `PredictSequential` is the honest
name; `BatchPredict` delegates to it and documents sequential execution in
the interface. Fused batching is Phase B3.

Acceptance: benchmark in `docs/benchmarks.md` showing tokens/sec for batch=1
vs batch=8, or the rename + doc.

### A6. Multi-input models (P1) ✅ 2026-10-14

`Predict` uses only the first input tensor ("Only support single input for
now" comment, ~L371).

Fix: accept `map[string]interface{}` keyed by input name; keep the
single-slice form as shorthand for single-input models.

Acceptance: test with a 2-input ONNX model.

### A7. Documentation sync (P1) ✅ 2026-10-14

README still says `/api/v1/infer` and `/api/v1/models/load` are "not
implemented yet"; `docs/API.md` documents mock responses (`mock_class_1`).
Rewrite README + `docs/API.md` from the actual router (`internal/api/router.go`
+ `handlers.go`). Delete anything describing behavior that doesn't exist.

Acceptance: a new user can go from README to a successful real prediction
without hitting a documented endpoint that lies.

### A8. Version-key validation (P2) ✅ 2026-10-14

Model keys are `id + ":" + version` by string concat. Reject IDs/versions
containing `:` (and empty IDs) at load time.

### A9. CLI/server API contract mismatch (P1) ✅ 2026-10-14

`cmd/goedgeinferctl` and the server disagree on every route. The CLI calls
`GET /models`, `POST /reload`, `POST /predict/:id`, `GET /remote_models`,
etc.; the router (`internal/api/router.go`) serves those under `/api/v1`
(`GET /api/v1/models`, `POST /api/v1/mgmt/system/reload`,
`POST /api/v1/infer/:model_id`, `GET /api/v1/mgmt/remote/models`). The CLI is
broken against its own server. Additionally, `runCLI` parses `--server` /
`--apikey` into a local FlagSet while `doRequest` reads the package-level
flags, so CLI flags are silently ignored (only env vars work).

Fix: point the CLI at the router's contract (the server is source of truth);
make `doRequest` use the parsed flag values. Add a contract test that boots
the server and runs every CLI command against it.

Acceptance: `goedgeinferctl --server http://localhost:8080 --apikey $KEY
list-models` (and every other command) succeeds against a live server.

---

## Phase B — Market relevance (target: 8–10 weeks solo)

### B1. GGUF / llama.cpp backend (highest leverage)

New file: `internal/inference/gguf_engine.go` implementing the existing
`Engine` interface via Go llama.cpp bindings (evaluate `go-llama.cpp` options;
pick the one with prebuilt binaries to avoid CGo hell for users).

Why first in Phase B: the edge-LLM world runs on GGUF. Without this, the
server speaks ONNX/TFLite while the market moved on. This is also where the
compressed-model thesis plugs in later.

Acceptance: serve a Q4_K_M 1B model; chat round-trip via the new API below;
documented memory footprint on a 4-vCPU box.

### B2. OpenAI-compatible API

Alongside the native API (don't break existing clients):

- `POST /v1/chat/completions` — streaming (SSE) and non-streaming
- `POST /v1/embeddings`
- `GET /v1/models`

Acceptance: the OpenAI Python SDK with `base_url` pointed at GoEdgeInfer
completes a chat and an embedding call unmodified. This is the ecosystem
compatibility that gets users (LangChain, Open WebUI, IDE plugins).

### B3. Real dynamic batching

Batches coalesce concurrent requests within a small window (10–50ms,
configurable) into one `session.Run` with batch dim; per-request latency SLO
config; falls back to immediate execution under low load.

Acceptance: benchmark — 8 concurrent clients show ≥4× throughput vs
sequential at p99 latency within SLO.

### B4. Model registry UX

`goedgeinferctl` + API: `pull` from Hugging Face (GGUF) as well as S3;
SHA256 verify on download (reuse `internal/model`); `list-remote` unified.

### B5. Persistent queue → opt-in

BadgerDB task durability becomes a mode flag (`--queue-mode=realtime|durable`,
default realtime). Durable mode is for offline batch jobs, not the hot path.

### B6. Publish benchmarks

`docs/benchmarks.md`: tokens/sec, TTFT, peak RSS for reference models
(quantized 1B/3B GGUF, a small ONNX classifier) on reference hardware
(4-vCPU VPS, e.g. Hetzner CX32-class). Re-run on every release. This file is
marketing material — treat it accordingly.

### B-gate (kill criteria — the grindsquad lesson)

After B1+B2+B6, answer with measurements, not vibes:

1. Can it serve a quantized 1–3B GGUF model on a $6–12/mo CPU VPS at
   ≥10 tok/s? If no, the cost thesis fails — stop or pivot before Phase C.
2. Does the OpenAI-compat API actually work against 3+ real clients
   (SDK, Open WebUI, one IDE plugin)? If integration keeps breaking, the
   compatibility story is fiction.

If either gate fails, do not proceed to Phase C. Fix, narrow scope, or shelve.

---

## Phase C — Revenue (target: 6–8 weeks solo)

Only after the B-gate passes.

### C1. Metering

Per-API-key usage accounting: tokens in/out, requests, compute-ms. SQLite by
default (single binary story), Postgres optional. `GET /v1/usage` per key.
This is the foundation everything else bills against.

### C2. Billing: Stripe + BTCPay

- Stripe: metered subscriptions (hobby free tier with hard caps, pro,
  scale). Webhook → quota updates.
- BTCPay Server: self-hosted (can share existing infra), 0% processing fees,
  Bitcoin + Lightning. Price in USD, settle in BTC, convert on a schedule.
  Crypto is an *option* at checkout, not the only rail.
- Entitlement service: key → plan → quotas → rate limiter (extend existing
  middleware from global to per-key).

### C3. Multi-tenancy hardening

Per-tenant rate limits and quotas (extend `internal/middleware/rate_limit.go`
from global to per-key), model access scoping per tenant, audit log of
admin actions (RBAC test file exists — wire it to real audit output).

### C4. Hosted beta

Deploy on a cheap VPS; onboarding docs (key issuance → first call in
<10 min); public status page; uptime SLO stated plainly.

### C5. Positioning page

One page that says the wedge in one sentence, e.g.: "metered inference for
quantized small models on CPU-only edge hardware." Benchmarks from B6 are
the proof. Name decision happens here, not before (current name is fine for
GitHub, weak as a brand).

### C-gate

8 weeks after beta opens: ≥5 paying or firmly committed users? If no,
distribution is the problem, not the code — do not keep building features.
Either find the channel or shelve.

---

## Cross-cutting

- CI (`.github/workflows`): `go vet`, `go test -race ./...`, `gofmt -l`,
  golangci-lint, plus the no-private-keys check (A3). Block merges on red.
- Keep the single-binary story: no required external services for the
  self-hosted path (SQLite default, BadgerDB optional).
- Every phase ends with docs updated *in the same PR*. Doc drift is how
  Phase A started; don't re-create it.

## Estimates

Solo, focused work: A = 2–3 weeks, B = 8–10 weeks, C = 6–8 weeks →
~4–6 months to a revenue-capable beta. Out-of-pocket until C4: ~$0
(domain + one VPS you likely already have). C4 adds ~$10–20/mo infra.
