package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"github.com/rusher2004/conways-api-go/bolt"
	"github.com/rusher2004/conways-api-go/server"
	"github.com/rusher2004/conways-api-go/store"
)

func run(ctx context.Context, dbPath, port string) error {
	// setup logger, db, and store dependencies
	handler := slog.NewJSONHandler(os.Stdout, nil)
	logger := slog.New(handler)

	db, err := bolt.NewConn(dbPath, "life")
	if err != nil {
		return fmt.Errorf("bolt: %w", err)
	}
	logger.Info("db opened")

	ls := store.NewLifeStore(db)
	srv := server.NewServer(ls, logger)

	// listen for interrupt signal to neatly shutdown server
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	httpServer := &http.Server{
		Addr:     ":" + port,
		Handler:  srv,
		ErrorLog: slog.NewLogLogger(handler, slog.LevelDebug),
	}

	go func() {
		logger.Info("server listening on port " + port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("error listening and serving", "error", err)
		}
	}()

	// use a waitgroup to block until we receive an interrupt signal
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		<-ctx.Done()
		logger.Info("shutting down server")

		downCtx := context.Background()
		downCtx, cancel := context.WithTimeout(downCtx, 5*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(downCtx); err != nil {
			logger.Error("error shutting down server", "error", err)
		}
	}()

	wg.Wait()

	return nil
}

func main() {
	ctx := context.Background()

	pwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting working directory: %v\n", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(pwd, "bolt.db")

	if err := run(ctx, dbPath, "8080"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
