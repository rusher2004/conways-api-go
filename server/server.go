package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rusher2004/conways-api-go/store"
	"github.com/rusher2004/conways-api-go/web"
)

type LifeStore interface {
	Create([][]bool) (int, error)
	Get(int, int) (board [][]bool, gen int, final bool, err error)
}

func errResponse(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	fmt.Fprintf(w, `{"error": "%s"}`, msg)
}

func NewServer(ls LifeStore, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	addRoutes(mux, ls, logger)

	return mux
}

func addRoutes(mux *http.ServeMux, ls LifeStore, logger *slog.Logger) {
	mux.Handle("POST /board", corsAllow(requestLogger(logger, dontPanic(logger, handleBoardPost(ls)))))
	mux.Handle("GET /board/{id}", corsAllow(requestLogger(logger, dontPanic(logger, handleBoardGet(ls)))))
	mux.Handle("GET /board/{id}/final", corsAllow(requestLogger(logger, dontPanic(logger, handleBoardGet(ls)))))
}

func handleBoardPost(ls LifeStore) http.Handler {
	type input struct {
		Cells [][]bool `json:"cells"`
	}

	type output struct {
		ID int `json:"id"`
	}

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			var in input
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				if errors.Is(err, io.EOF) {
					errResponse(w, "missing body", http.StatusBadRequest)
					return
				}

				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			// some validation
			// cells must be present and non-empty
			if in.Cells == nil {
				errResponse(w, "missing cells", http.StatusBadRequest)
				return
			}

			if len(in.Cells) == 0 {
				errResponse(w, "empty cells", http.StatusBadRequest)
				return
			}

			id, err := ls.Create(in.Cells)
			if err != nil {
				errResponse(w, err.Error(), http.StatusInternalServerError)
				return
			}

			out := output{ID: id}

			w.WriteHeader(http.StatusCreated)
			if err := json.NewEncoder(w).Encode(out); err != nil {
				errResponse(w, err.Error(), http.StatusInternalServerError)
				return
			}
		},
	)
}

func handleBoardGet(ls LifeStore) http.Handler {
	type output struct {
		ID         int      `json:"id"`
		Cells      [][]bool `json:"cells"`
		Generation int      `json:"generation"`
		Final      bool     `json:"final"`
	}

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// board id from path
			id := r.PathValue("id")
			idInt, err := strconv.Atoi(id)
			if err != nil {
				errResponse(w, "invalid id", http.StatusBadRequest)
				return
			}

			finalRequired := strings.HasSuffix(r.URL.Path, "/final")

			// optional query param for state
			params := r.URL.Query()
			state := 0
			if params.Has("state") {
				stateParam := params.Get("state")
				stateInt, err := strconv.Atoi(stateParam)
				if err != nil {
					errResponse(w, "invalid state", http.StatusBadRequest)
					return
				}
				// default to 1000 if state is too large
				state = min(1000, stateInt)
			}

			if finalRequired && state < 1 {
				errResponse(w, "state must be greater than 0", http.StatusBadRequest)
				return
			}

			board, gen, final, err := ls.Get(idInt, state)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					errResponse(w, "not found", http.StatusNotFound)
					return
				}
				errResponse(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if finalRequired && !final {
				errResponse(w, "final state not reached", http.StatusBadRequest)
				return
			}

			if strings.Contains(r.Header.Get("Accept"), "text/html") {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				web.BoardView(idInt, board, gen, final).Render(r.Context(), w)
				return
			}

			out := output{
				ID:         idInt,
				Cells:      board,
				Generation: gen,
				Final:      final,
			}

			if err := json.NewEncoder(w).Encode(out); err != nil {
				errResponse(w, err.Error(), http.StatusInternalServerError)
				return
			}
		},
	)
}

// dontPanic is a middleware to ensure that the server does not crash due to a panic in the handler h.
// If a recover is triggered, it will log the error and return a 500 status code.
func dontPanic(logger *slog.Logger, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				logger.Error("recovered from panic", "error", rvr)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()

		h.ServeHTTP(w, r)
	})
}

// requestLogger is a middleware to log the request and response details.
func requestLogger(logger *slog.Logger, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()

		logger.Info("request", "method", r.Method, "url", r.URL.String())
		defer func() {
			logger.Info("response", "url", r.URL.String(), "duration", time.Since(now).String())
		}()

		h.ServeHTTP(w, r)
	})
}

// corsAllow is a middleware to allow the OpenAPI Editor to make requests.
func corsAllow(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")

		h.ServeHTTP(w, r)
	})
}
