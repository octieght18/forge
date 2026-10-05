package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/octieght18/forge/internal/config"
	"github.com/octieght18/forge/internal/httpapi"
	"github.com/octieght18/forge/internal/service"
)

func main() { os.Exit(run()) }

func run() int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	c, err := config.Load(os.LookupEnv)
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		return 1
	}
	readiness := &httpapi.Readiness{}
	server, err := service.New(c, logger, readiness, httpapi.NewHandler(logger, readiness))
	if err != nil {
		logger.Error("server initialization failed", "error", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	listener, err := net.Listen("tcp", c.Address)
	if err != nil {
		logger.Error("listen failed", "error", err)
		return 1
	}
	if err := server.Run(ctx, listener); err != nil {
		logger.Error("API stopped with error", "error", err)
		return 1
	}
	return 0
}
