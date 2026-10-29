package agent

import (
	"encoding/binary"
	"encoding/json"
	"time"

	bolt "go.etcd.io/bbolt"
)

var outboxBucket = []byte("outbox")

// Outbox is a durable FIFO of messages waiting for the cloud. Events are
// written here first, so a site that loses its internet keeps every event
// and sends it when the link comes back.
type Outbox struct {
	db *bolt.DB
}

type outboxItem struct {
	Topic   string `json:"t"`
	Payload []byte `json:"p"`
}

func OpenOutbox(path string) (*Outbox, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(outboxBucket)
		return err
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Outbox{db: db}, nil
}

func (o *Outbox) Close() error { return o.db.Close() }

// Push appends a message.
func (o *Outbox) Push(topic string, payload []byte) error {
	return o.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(outboxBucket)
		seq, err := b.NextSequence()
		if err != nil {
			return err
		}
		v, err := json.Marshal(outboxItem{topic, payload})
		if err != nil {
			return err
		}
		return b.Put(seqKey(seq), v)
	})
}

// Drain hands queued messages to send in order and deletes each one that
// send accepts. It stops at the first failure so ordering is kept.
func (o *Outbox) Drain(limit int, send func(topic string, payload []byte) error) (int, error) {
	type entry struct {
		key  []byte
		item outboxItem
	}
	var batch []entry
	err := o.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(outboxBucket).Cursor()
		for k, v := c.First(); k != nil && len(batch) < limit; k, v = c.Next() {
			var it outboxItem
			if err := json.Unmarshal(v, &it); err != nil {
				continue
			}
			batch = append(batch, entry{append([]byte(nil), k...), it})
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, e := range batch {
		if err := send(e.item.Topic, e.item.Payload); err != nil {
			return sent, err
		}
		if err := o.db.Update(func(tx *bolt.Tx) error {
			return tx.Bucket(outboxBucket).Delete(e.key)
		}); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
}

// Len returns the number of queued messages.
func (o *Outbox) Len() int {
	n := 0
	_ = o.db.View(func(tx *bolt.Tx) error {
		n = tx.Bucket(outboxBucket).Stats().KeyN
		return nil
	})
	return n
}

func seqKey(seq uint64) []byte {
	k := make([]byte, 8)
	binary.BigEndian.PutUint64(k, seq)
	return k
}
