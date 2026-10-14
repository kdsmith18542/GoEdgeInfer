# GoEdgeInfer

A metered, multi-tenant inference server for quantized small models on
CPU-only edge hardware. Single Go binary: load a model, serve predictions over
HTTP or gRPC, pay per use.

> **Positioning (2026-10 pivot):** this is not another general inference
> server. The product is the SaaS-ification — metering, tenancy, billing, and
> an OpenAI-compatible API — over small/quantized models where a $6 VPS beats
> a GPU box. See `plan.md` and `docs/roadmap.md`.

## What works today

- **Engines:** ONNX Runtime (real inference, CPU + optional CUDA), TensorFlow
  Lite (cgo + mock). GGUF/llama.cpp backend is planned (see `plan.md` B1).
- **Model management:** versioned load/unload, SHA256 integrity check, ECDSA
  signature verification, S3/MinIO registry, Hugging Face pulls planned.
- **Serving:** HTTP REST (`/api/v1`), gRPC (`GoEdgeInferService`), worker pool
  with optional BadgerDB durable queue, dynamic batching planned.
- **Ops:** API-key + JWT + RBAC auth, per-key rate limiting, Prometheus
  metrics, OpenTelemetry tracing, `goedgeinferctl` CLI, Docker image.

## Quickstart

```bash
# 1. ONNX Runtime library (or set ORT_LIB_PATH to your install)
export ORT_LIB_PATH=/usr/local/lib/libonnxruntime.so

# 2. Run
go run ./cmd --config configs/config.yaml
# or: docker build -t goedgeinfer . && docker run -p 8080:8080 goedgeinfer

# 3. Load a model (needs X-API-Key, see configs/config.yaml)
curl -X POST localhost:8080/api/v1/models \
  -H 'X-API-Key: $KEY' -H 'Content-Type: application/json' \
  -d '{"model_id":"mnist","model_path":"/models/mnist.onnx"}'

# 4. Predict
curl -X POST localhost:8080/api/v1/infer/mnist \
  -H 'X-API-Key: $KEY' -H 'Content-Type: application/json' \
  -d '{"input":[0.1,0.2,0.3]}'
```

Full route table: `docs/API.md`. CLI usage: `docs/cli.md`.

## API surface (current)

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/health`, `/metrics` | none | liveness, Prometheus |
| GET/POST | `/api/v1/models…` | API key | list / load / info / unload (versioned) |
| POST | `/api/v1/infer/:model_id[​/versions/:v]` | API key | predict |
| POST | `/api/v1/infer/batch/:model_id[​/versions/:v]` | API key | batch predict |
| * | `/api/v1/mgmt/…` | JWT | remote registry, cache, reload, config |
| gRPC | `GoEdgeInferService.Infer`, `StreamInfer` | API key | unary + streaming |

Planned (Phase B): `POST /v1/chat/completions` (SSE + non-stream),
`POST /v1/embeddings`, `GET /v1/models` — OpenAI-compatible.

## Configuration

`configs/config.yaml` (or env vars — see `internal/config`). Key settings:

- `server.port`, `server.api_key`
- `onnxruntime.lib_path` (or `ORT_LIB_PATH`) — **required**, no hardcoded default
- `execution_provider.type` — `cpu` (default) or `cuda`
- `queue.mode` — `realtime` (default) or `durable`
- `signature_verification.enabled`, `signature_verification.public_key_pem`
- `s3.*` — remote model registry

## Repository layout

```
cmd/                  server entrypoint, goedgeinferctl CLI
internal/
  api/                HTTP handlers + router (gin)
  grpcapi/            gRPC server
  inference/          Engine interface; onnxruntime, tflite, mock engines
  model/              ModelManager: versioning, SHA256/ECDSA, S3 download
  worker/             goroutine pool
  persistence/        BadgerDB durable queue (opt-in)
  processing/         pre/post-processing pipeline
  middleware/         api key, JWT, RBAC, rate limit, request id
  security/          model signature verification
  config/             viper config
  app/                server wiring/lifecycle
pkg/logging pkg/metrics
proto/infer.proto
configs/              config.yaml (NO secrets — see SECURITY.md)
docs/                 specs (see docs/roadmap.md for what's current)
scripts/              test helpers
test_models/          fixture models
```

## Development

```bash
go build ./...
go test -race ./...        # -race is mandatory; see plan.md A1
gofmt -l .
```

CI blocks merges on `go vet`, `-race` tests, and a no-private-keys check.

## Docs

- `plan.md` — phased build plan with kill gates (start here)
- `GoEdgeInfer.md` — architecture blueprint (revised 2026-10)
- `docs/roadmap.md` — done / now / next
- `docs/API.md` — full route reference
- `docs/architecture.md` — components, concurrency model, backends
- `docs/billing.md` — metering + Stripe/BTCPay design (planned)
- `SECURITY.md` — secret handling, auth model

## License

MIT. See LICENSE.
