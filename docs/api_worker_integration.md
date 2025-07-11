# API and Worker Pool Integration in GoEdgeInfer

## Overview
GoEdgeInfer's HTTP API and gRPC API are tightly integrated with a worker pool and a persistent queue to ensure reliable, concurrent, and durable inference request processing.

## Flow
1. **API Layer**
   - Receives inference requests via HTTP (`/predict/:model_id`) or gRPC.
   - Validates and parses the request.
   - Submits the request to the worker pool (which also persists it to disk).
   - Waits for the result or error from the worker pool and returns the response.

2. **Worker Pool**
   - Manages a pool of goroutines for concurrent inference execution.
   - Uses an in-memory channel for fast task dispatch.
   - Optionally, persists all tasks to a disk-backed queue (BadgerDB) for durability.
   - On startup, recovers and processes any unprocessed tasks from the persistent queue.

3. **Persistent Queue**
   - Ensures no inference request is lost, even if the agent is restarted or crashes.
   - All tasks are enqueued to disk before being processed by the worker pool.

## Key Files
- API handlers: `internal/api/handlers.go`
- Worker pool: `internal/worker/pool.go`
- Persistent queue: `internal/persistence/queue.go`
- Integration test: `internal/api/handlers_test.go`

## Example HTTP Flow
1. Client POSTs to `/predict/my_model` with input data.
2. API handler submits the request to the worker pool.
3. Worker pool enqueues the task to the persistent queue and in-memory channel.
4. Worker goroutine processes the task, runs inference, and returns the result.
5. API handler responds to the client with the inference result.

## Durability
- If the agent is restarted, any unprocessed tasks in the persistent queue are recovered and processed automatically.

## Testing
- Integration tests verify that requests are handled, persisted, and processed correctly.

## Extensibility
- The same pattern applies to gRPC API integration.
- The persistent queue can be extended to support result buffering, batching, or advanced retry logic.

---
For more, see `GoEdgeInfer.md` and the code comments in each module.
