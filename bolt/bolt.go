package bolt

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/rusher2004/conways-api-go/store"
	"go.etcd.io/bbolt"
)

type board struct {
	Cells      [][]bool `json:"cells"`
	Generation int      `json:"generation"`
	Final      bool     `json:"final"`
}

type Conn struct {
	bucket string
	db     *bbolt.DB
}

func NewConn(path, bucket string) (*Conn, error) {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucket))
		if err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}

		return nil
	})

	return &Conn{
		bucket: bucket,
		db:     db,
	}, nil
}

func (c *Conn) Close() error {
	return c.db.Close()
}

func (c *Conn) Create(input [][]bool) (int, error) {
	var out uint64

	boltBoard := board{
		Cells:      input,
		Generation: 0,
	}

	if err := c.db.Update(func(tx *bbolt.Tx) error {
		b, err := json.Marshal(boltBoard)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}

		bucket := tx.Bucket([]byte(c.bucket))
		id, err := bucket.NextSequence()
		if err != nil {
			return fmt.Errorf("next sequence: %w", err)
		}

		if err := bucket.Put([]byte(strconv.FormatUint(id, 10)), b); err != nil {
			return fmt.Errorf("put: %w", err)
		}

		out = id
		return nil
	}); err != nil {
		return -1, fmt.Errorf("update: %w", err)
	}

	return int(out), nil
}

func (c *Conn) Get(id int) (store.Board, error) {
	var out board
	if err := c.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(c.bucket))

		res := bucket.Get([]byte(strconv.Itoa(id)))
		if res == nil {
			return store.ErrNotFound
		}

		if err := json.Unmarshal(res, &out); err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}

		return nil
	}); err != nil {
		return store.Board{}, fmt.Errorf("view: %w", err)
	}

	return store.Board(out), nil
}

func (c *Conn) Save(id int, input store.Board) error {
	boltBoard := board(input)

	if err := c.db.Update(func(tx *bbolt.Tx) error {
		b, err := json.Marshal(boltBoard)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}

		bucket := tx.Bucket([]byte(c.bucket))
		if err := bucket.Put([]byte(strconv.Itoa(id)), b); err != nil {
			return fmt.Errorf("put: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("update: %w", err)
	}

	return nil
}
