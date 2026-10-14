# Metrics & Observability

## Prometheus (`GET /metrics`, no auth)

Per-model labels are `model_id` (and `version` where recorded):

- `goedgeinfer_inference_requests_total{model_id}` — counter
- `goedgeinfer_inference_duration_seconds{model_id}` — histogram
- `goedgeinfer_inference_errors_total{model_id}` — counter
- `goedgeinfer_model_load_total{model_id,version,status}` — counter
- `goedgeinfer_model_load_duration_seconds{model_id,version}` — histogram
- `goedgeinfer_model_unload_total{model_id,version,status}` — counter
- `goedgeinfer_queue_depth` — gauge (durable mode)
- `goedgeinfer_tokens_in_total{api_key_id,model_id}` — counter **[planned, C1]**
- `goedgeinfer_tokens_out_total{api_key_id,model_id}` — counter **[planned, C1]**

System collectors: `gopsutil` is a dependency — CPU/mem/disk collectors are
wired, not "planned" (the old doc was stale).

## Tracing

OpenTelemetry is implemented: model load/unload and inference create spans
(`ModelManager.LoadModel`, engine Predict paths). Exporter is stdout by
default; point it at your collector via standard OTel env vars.

## Health

`GET /health` — liveness/readiness probe (no auth, no rate limit).

## Benchmarks

`docs/benchmarks.md` (planned, B6): tokens/sec, TTFT, peak RSS for reference
models on reference hardware, re-run per release. Marketing-grade — keep it
honest.

## Extending

Add per-endpoint latency histograms before adding new metric types. Cardinality
discipline: `api_key_id` labels only on the token counters, never raw keys.
