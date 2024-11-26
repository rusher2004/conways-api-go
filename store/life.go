package store

import (
	"errors"
	"fmt"

	"github.com/rusher2004/conways-api-go/life"
)

type Board struct {
	Cells      [][]bool
	Generation int
	Final      bool
}

type DB interface {
	Create(board [][]bool) (int, error)
	Get(id int) (Board, error)
	Save(id int, board Board) error
}

type LifeStore struct {
	db DB
}

var ErrNotFound = errors.New("not found")

func NewLifeStore(db DB) *LifeStore {
	return &LifeStore{db}
}

func (ls *LifeStore) Create(board [][]bool) (int, error) {
	id, err := ls.db.Create(board)
	if err != nil {
		return -1, fmt.Errorf("save: %w", err)
	}

	return id, nil
}

func (ls *LifeStore) Get(id int, generations int) ([][]bool, int, bool, error) {
	board, err := ls.db.Get(id)
	if err != nil {
		return nil, -1, false, fmt.Errorf("get: %w", err)
	}

	if generations == 0 || board.Final {
		return board.Cells, board.Generation, board.Final, nil
	}

	for range generations {
		board.Cells, board.Final = life.NextGeneration(board.Cells)
		board.Generation++
		if board.Final {
			break
		}
	}

	if err := ls.db.Save(id, board); err != nil {
		return nil, -1, false, fmt.Errorf("save: %w", err)
	}

	return board.Cells, board.Generation, board.Final, nil
}
