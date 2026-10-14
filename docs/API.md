# GoEdgeInfer API Reference

Base URL: `http://localhost:8080`. All `/api/v1/*` paths (unless noted) require
the `X-API-Key` header and are rate-limited (default 10 rps, burst 20; per-key
quotas planned). Every response carries an `X-Request-ID`.

> Verified against `internal/api/router.go`. The old version of this doc
> described mock responses — those were replaced.

## System

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/health` | none | liveness probe |
| GET | `/api/v1/public/health` | none | same, under versioned prefix |
| GET | `/metrics` | none | Prometheus scrape |

## Models

| Method | Path | Body | Notes |
|---|---|---|---|
| GET | `/api/v1/models` | — | list loaded `id:version` keys |
| POST | `/api/v1/models` | `{"model_id","model_path","version?"}` | `model_path` may be `s3://bucket/key`; SHA256 verified, ECDSA sig verified if enabled |
| GET | `/api/v1/models/:model_id` | — | info (latest or unversioned) |
| DELETE | `/api/v1/models/:model_id` | — | unload |
| GET | `/api/v1/models/:model_id/versions/:version` | — | info for a version |
| DELETE | `/api/v1/models/:model_id/versions/:version` | — | unload a version |

Model IDs and versions must not contain `:`.

## Inference

| Method | Path | Body |
|---|---|---|
| POST | `/api/v1/infer/:model_id` | `{"input":[…] \| {name:[…],…}, "version?":…}` |
| POST | `/api/v1/infer/:model_id/versions/:version` | same |
| POST | `/api/v1/infer/batch/:model_id` | `{"inputs":[…]}` |
| POST | `/api/v1/infer/batch/:model_id/versions/:version` | same |

Response:

```json
{
  "model_id": "mnist",
  "output": { "data": [0.01, 0.97, 0.02], "shape": [3], "model_id": "mnist" },
  "metadata": { "engine": "onnxruntime", "latency_ms": 4 },
  "latency_ms": 4
}
```

`input` accepts `[]float32` (flat), `[][]float32` (flattened), `[]float64`,
`[]int8`/`[]uint8`/`[]int32`/`[]int64`, or `[]bool` (converted to the tensor
dtype read from the model). For multi-input models pass a map keyed by input
tensor name: `{"input": {"x": […], "mask": […]}}` — a bare slice is shorthand
for single-input models. Quantized int8/uint8 tensors load and run (plan.md
A4).

**Batching note:** `/infer/batch/:model_id` runs inputs **sequentially**, one
session run each — it is not a fused batch and gains no throughput over
concurrent single predicts. Real dynamic batching is plan.md B3.

## Management (`/api/v1/mgmt`, JWT when enabled)

| Method | Path | Notes |
|---|---|---|
| GET | `/mgmt/remote/models` | list remote registry |
| POST | `/mgmt/remote/upload` | `{"local_path","object_key"}` |
| POST | `/mgmt/remote/delete` | `{"object_key"}` — irreversible |
| POST | `/mgmt/remote/cleanup` | `{"cache_dir","keep":[…]}` |
| POST | `/mgmt/system/reload` | reload config/models |
| GET | `/mgmt/system/config` | admin role only |

## gRPC

`proto/infer.proto` — `GoEdgeInferService`:

- `Infer(InferRequest) → InferResponse` (unary)
- `StreamInfer(stream InferRequest) → stream InferResponse` (bidi, for
  audio/video frame streams)

## OpenAI-compatible [PLANNED — Phase B]

| Method | Path | Notes |
|---|---|---|
| POST | `/v1/chat/completions` | SSE streaming + non-stream |
| POST | `/v1/embeddings` | |
| GET | `/v1/models` | |

Acceptance: OpenAI Python SDK works with only `base_url` changed.

## Errors

`400` malformed input · `401` missing/invalid API key · `403` JWT role
insufficient · `404` model not found · `429` rate limited · `500` inference
failure. All errors are JSON: `{"error":"…"}`.
