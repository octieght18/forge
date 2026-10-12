package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/octieght18/forge/internal/execution"
	"github.com/octieght18/forge/internal/reconcile"
	"github.com/octieght18/forge/internal/store"
	"go.temporal.io/sdk/client"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	dsn := os.Getenv("FORGE_DISPATCHER_DATABASE_URL")
	if dsn == "" {
		logger.Error("dispatcher database configuration required")
		os.Exit(1)
	}
	address := os.Getenv("TEMPORAL_ADDRESS")
	if address == "" {
		address = "127.0.0.1:7233"
	}
	namespace := os.Getenv("TEMPORAL_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		logger.Error("dispatcher database configuration failed")
		os.Exit(1)
	}
	defer pool.Close()
	queue, err := store.NewDispatcher(pool)
	if err != nil {
		logger.Error("dispatcher initialization failed")
		os.Exit(1)
	}
	service, err := client.Dial(client.Options{HostPort: address, Namespace: namespace})
	if err != nil {
		logger.Error("temporal client failed")
		os.Exit(1)
	}
	defer service.Close()
	loop, err := reconcile.New(queue, execution.Backend{Client: execution.SDK{Client: service}}, reconcile.Defaults())
	if err != nil {
		logger.Error("reconciler initialization failed")
		os.Exit(1)
	}
	if err = loop.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("dispatcher stopped")
		os.Exit(1)
	}
}
