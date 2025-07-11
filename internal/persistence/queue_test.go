package persistence

import (
	"os"
	"testing"
)

type testItem struct {
	ID   string
	Data string
}

func TestPersistentQueue_EnqueueDequeue(t *testing.T) {
	os.RemoveAll("./testdata/queue_test")
	queue, err := NewPersistentQueue("./testdata/queue_test")
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer queue.Close()

	item := testItem{ID: "1", Data: "foo"}
	if err := queue.Enqueue(item); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	var out testItem
	if err := queue.Dequeue(&out); err != nil {
		t.Fatalf("dequeue failed: %v", err)
	}
	if out.ID != item.ID || out.Data != item.Data {
		t.Errorf("dequeued item mismatch: got %+v, want %+v", out, item)
	}
}
