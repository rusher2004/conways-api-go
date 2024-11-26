package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/bbolt"
)

type board struct {
	Cells      [][]bool `json:"cells"`
	Generation int      `json:"generation"`
	Final      bool     `json:"final"`
}

var boards = []board{
	{
		// simple board to fetch
		Cells:      [][]bool{{true}},
		Generation: 0,
		Final:      false,
	},
	{
		// blinker, to predict any arbitrary generation
		Cells: [][]bool{
			{false, false, false, false, false},
			{false, false, false, false, false},
			{false, true, true, true, false},
			{false, false, false, false, false},
			{false, false, false, false, false},
		},
		Generation: 0,
		Final:      false,
	},
	{
		// arbitrary board with a final generation
		Cells: [][]bool{
			{false, false, false, false, false},
			{false, false, true, false, false},
			{false, true, true, true, false},
			{false, false, false, true, false},
			{false, false, false, false, false},
		},
		Generation: 0,
		Final:      false,
	},
	{
		// blinker, to be used to test final state error
		Cells: [][]bool{
			{false, false, false, false, false},
			{false, false, false, false, false},
			{false, true, true, true, false},
			{false, false, false, false, false},
			{false, false, false, false, false},
		},
		Generation: 0,
		Final:      false,
	},
}

type testCase struct {
	name      string
	id        string
	method    string
	query     string
	reqBody   string
	extraPath string // hacky addition to test final state. would be cleaned up.

	wantStatus int
	wantBody   string
}

func setup(dbPath string) (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}

	tempPath := filepath.Join(pwd, dbPath)

	if err := setupDB(tempPath); err != nil {
		return "", fmt.Errorf("setup db: %w", err)
	}

	return tempPath, nil
}

func setupDB(path string) error {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer db.Close()

	if err := db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("life"))
		if err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}

		for _, cells := range boards {
			id, err := bucket.NextSequence()
			if err != nil {
				return fmt.Errorf("next sequence: %w", err)
			}

			b, err := json.Marshal(cells)
			if err != nil {
				return fmt.Errorf("marshal: %w", err)
			}

			if err := bucket.Put([]byte(fmt.Sprintf("%d", id)), b); err != nil {
				return fmt.Errorf("put: %w", err)
			}
		}

		return nil

	}); err != nil {
		return fmt.Errorf("update: %w", err)
	}

	return nil
}

func runTest(t *testing.T, baseURL string, tc testCase, a *assert.Assertions, cl http.Client) {
	t.Run(tc.name, func(t *testing.T) {
		elems := []string{"board", tc.id}
		if tc.extraPath != "" {
			elems = append(elems, tc.extraPath)
		}
		// reqPath, err := url.JoinPath("http://localhost:8181/board", tc.id)
		reqPath, err := url.JoinPath(baseURL, elems...)
		if err != nil {
			t.Fatalf("error joining path: %v", err)
		}

		var buf io.Reader
		// create a request body if needed
		if tc.reqBody != "" {
			buf = strings.NewReader(tc.reqBody)
		}

		// add query params if needed
		if tc.query != "" {
			reqPath += "?" + tc.query
		}

		req, err := http.NewRequest(tc.method, reqPath, buf)
		if err != nil {
			t.Fatalf("error creating request: %v", err)
		}

		// make http request
		res, err := cl.Do(req)
		if err != nil {
			t.Fatalf("error making request: %v", err)
		}
		defer res.Body.Close()

		// check http status
		a.Equal(tc.wantStatus, res.StatusCode)

		// check contents of body
		b, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("error reading body: %v", err)
		}

		var out map[string]any
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("error unmarshalling body: %v", err)
		}

		var want map[string]any
		if err := json.Unmarshal([]byte(tc.wantBody), &want); err != nil {
			t.Fatalf("error unmarshalling want body: %v", err)
		}

		a.Equal(want, out)
	})
}

func TestRun(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	tempDBPath := "../tmp/temp.db"

	dbPath, err := setup(tempDBPath)
	if err != nil {
		t.Fatalf("error setting up db: %v", err)
	}

	t.Cleanup(func() {
		cancel()

		if err := os.Remove(dbPath); err != nil {
			t.Fatalf("error removing temp db: %v", err)
		}
	})

	// start the server
	port := "8181"
	go run(ctx, dbPath, port)

	// wait for server to start. realistically, we'd poll on a health endpoint. but there is no heavy
	// setup of remote dependencies in this app, so waiting should be fine.
	time.Sleep(1 * time.Second)

	// test cases
	tests := []testCase{
		// happy path
		{
			name:       "fetch existing board",
			id:         "1",
			method:     "GET",
			wantStatus: 200,
			wantBody:   `{"id":1,"cells":[[true]],"generation":0,"final":false}`,
		},
		{
			name:       "create new board",
			method:     "POST",
			reqBody:    `{"cells":[[true, false]]}`,
			wantStatus: 201,
			wantBody:   fmt.Sprintf(`{"id":%d}`, len(boards)+1),
		},
		{
			name:       "fetch blinker board, advance 1 generation",
			id:         "2",
			method:     "GET",
			query:      "state=1",
			wantStatus: 200,
			wantBody:   `{"id":2,"cells":[[false,false,false,false,false],[false, false, true, false, false],[false, false, true, false, false],[false, false, true, false, false],[false,false,false,false,false]],"generation":1,"final":false}`,
		},
		{
			name:       "fetch blinker board, advance to an arbitrary even number of generations",
			id:         "2",
			method:     "GET",
			query:      "state=75",
			wantStatus: 200,
			wantBody:   `{"id":2,"cells":[[false,false,false,false,false],[false,false,false,false,false],[false, true, true, true, false],[false,false,false,false,false],[false,false,false,false,false]],"generation":76,"final":false}`,
		},
		{
			name:       "fetch a board, requesting state past its final generation. should return final state and correct generation",
			id:         "3",
			method:     "GET",
			query:      "state=100",
			wantStatus: 200,
			wantBody:   `{"id":3,"cells":[[false,false,false,false,false],[false,false,false,false,false],[false,false,false,false,false],[false,false,false,false,false],[false,false,false,false,false]],"generation":14,"final":true}`,
		},
		{
			name:       "fetch blinker board, requesting final state. should return error because final state cannot be reached",
			id:         "4",
			method:     "GET",
			query:      "state=1000",
			extraPath:  "final",
			wantStatus: 400,
			wantBody:   `{"error":"final state not reached"}`,
		},
	}

	cl := http.Client{Timeout: 5 * time.Second}

	a := assert.New(t)
	baseURL := "http://localhost:" + port
	for _, tt := range tests {
		runTest(t, baseURL, tt, a, cl)
	}
}
