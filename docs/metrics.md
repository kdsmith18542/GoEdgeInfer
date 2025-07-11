# GoEdgeInfer Metrics & Observability

## Prometheus Metrics
GoEdgeInfer exposes a `/metrics` endpoint (HTTP) for Prometheus scraping. Metrics are implemented using the Prometheus Go client and include per-model and system-level statistics.

### Exposed Metrics
- `goedgeinfer_inference_requests_total{model_id}`: Total number of inference requests per model.
- `goedgeinfer_inference_duration_seconds{model_id}`: Histogram of inference latency per model (seconds).
- `goedgeinfer_inference_errors_total{model_id}`: Total number of inference errors per model.
- `goedgeinfer_queue_depth`: Current depth of the persistent inference request queue.

### Example Prometheus Scrape
```
# HELP goedgeinfer_inference_requests_total The total number of inference requests (by model)
# TYPE goedgeinfer_inference_requests_total counter
goedgeinfer_inference_requests_total{model_id="mnist"} 42
# HELP goedgeinfer_inference_duration_seconds Duration of inference requests in seconds (by model)
# TYPE goedgeinfer_inference_duration_seconds histogram
...
# HELP goedgeinfer_queue_depth Current depth of the inference request queue
# TYPE goedgeinfer_queue_depth gauge
goedgeinfer_queue_depth 0
```

## System Health (Planned/Extensible)
- CPU, memory, and disk usage can be added using [prometheus/client_golang](https://github.com/prometheus/client_golang) collectors or [github.com/shirou/gopsutil](https://github.com/shirou/gopsutil).
- Health endpoint: `/health` returns a simple status for liveness/readiness probes.

## How to Use
1. Start GoEdgeInfer.
2. Point Prometheus to `http://<host>:<port>/metrics`.
3. Use Grafana or Prometheus UI to visualize metrics.

## Extending Observability
- Add custom metrics for batch size, model load/unload times, or hardware utilization.
- Integrate OpenTelemetry for distributed tracing (see blueprint for future work).

---
See also: `docs/api_worker_integration.md`, `GoEdgeInfer.md` for more on architecture and observability.
