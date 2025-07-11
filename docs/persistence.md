# Persistent Queue (BadgerDB) in GoEdgeInfer

## Overview
The persistent queue is a disk-backed FIFO queue implemented using BadgerDB. It ensures that inference requests are not lost across restarts or failures, providing store-and-forward capability for edge scenarios.

## Location
- Implementation: `internal/persistence/queue.go`
- Tests: `internal/persistence/queue_test.go`

## Usage
- The queue is initialized in `main.go` and passed to the worker pool.
- All inference tasks are enqueued to disk on submission.
- On startup, any unprocessed tasks are recovered and re-enqueued for processing.

## API
```
NewPersistentQueue(path string) (*PersistentQueue, error)
Enqueue(item interface{}) error
Dequeue(out interface{}) error
Close() error
```

## Example
```
queue, _ := persistence.NewPersistentQueue("./data/queue")
queue.Enqueue(MyTask{...})
var task MyTask
queue.Dequeue(&task)
queue.Close()
```

## Integration
- The worker pool uses the persistent queue for all inference tasks.
- The API and gRPC endpoints are automatically durable.

## Test
Run:
```
go test ./internal/persistence/...
```

## Notes
- The queue stores items as JSON blobs for generality.
- The queue directory should be on persistent storage for durability.
