package persistence

import (
	"encoding/json"
	"sync"

	"github.com/dgraph-io/badger/v4"
)

// PersistentQueue is a disk-backed FIFO queue for inference requests/results
// Each item is stored as a JSON blob for generality

type PersistentQueue struct {
	db   *badger.DB
	lock sync.Mutex
	seq  uint64 // Monotonically increasing sequence for FIFO
}

func NewPersistentQueue(path string) (*PersistentQueue, error) {
	db, err := badger.Open(badger.DefaultOptions(path).WithLogger(nil))
	if err != nil {
		return nil, err
	}
	return &PersistentQueue{db: db, seq: 0}, nil
}

func (q *PersistentQueue) Enqueue(item interface{}) error {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.seq++
	key := make([]byte, 8)
	for i := 0; i < 8; i++ {
		key[7-i] = byte(q.seq >> (i * 8))
	}
	val, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return q.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
	})
}

func (q *PersistentQueue) Dequeue(out interface{}) error {
	q.lock.Lock()
	defer q.lock.Unlock()
	var valCopy []byte
	err := q.db.Update(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		it.Rewind()
		if !it.Valid() {
			return badger.ErrKeyNotFound
		}
		item := it.Item()
		val, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		valCopy = val
		return txn.Delete(item.Key())
	})
	if err != nil {
		return err
	}
	return json.Unmarshal(valCopy, out)
}

func (q *PersistentQueue) Len() (int, error) {
	q.lock.Lock()
	defer q.lock.Unlock()
	count := 0
	err := q.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			count++
		}
		return nil
	})
	if err != nil {
		return -1, err
	}
	return count, nil
}

func (q *PersistentQueue) Close() error {
	return q.db.Close()
}
