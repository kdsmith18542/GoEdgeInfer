# Persistent Queue (BadgerDB) — queue modes

> **Pivot (2026-10):** durability used to be unconditional — every inference
> request hit disk. It's now opt-in. Realtime is the default.

## Modes (`queue.mode` in config)

| Mode | Behavior | Use for |
|---|---|---|
| `realtime` (default) | in-memory channel dispatch; failures return to caller immediately | live serving, edge inference |
| `durable` | requests written to BadgerDB before dispatch; unprocessed tasks recovered on startup | offline batch jobs, store-and-forward on flaky networks |

## Implementation

- `internal/persistence/queue.go` — `NewPersistentQueue(path)`,
  `Enqueue` / `Dequeue` / `Close`; items stored as JSON blobs.
- `internal/worker/pool.go` — consumes the queue only in `durable` mode.
- Queue directory must be on persistent storage. Exposed metric:
  `goedgeinfer_queue_depth`.

## Rationale

Disk on the hot path is a latency tax. Most inference traffic wants fast
failure + client retry, not store-and-forward. Durability is a feature for
batch workloads — so it's a mode, not the architecture.

## Test

```bash
go test ./internal/persistence/... ./internal/worker/...
```
