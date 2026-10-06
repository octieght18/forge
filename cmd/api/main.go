package main

import (
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/octieght18/forge/internal/auth"
	"github.com/octieght18/forge/internal/config"
	"github.com/octieght18/forge/internal/httpapi"
	"github.com/octieght18/forge/internal/service"
	"github.com/octieght18/forge/internal/store"
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
	product, err := config.LoadProduct(os.LookupEnv)
	if err != nil {
		logger.Error("invalid product configuration", "error", err)
		return 1
	}
	handler := httpapi.NewHandler(logger, readiness)
	if product.Enabled {
		if product.Timeout >= c.WriteTimeout || product.Timeout > c.ReadTimeout {
			logger.Error("operation timeout must fit HTTP read/write timeouts")
			return 1
		}
		startup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		pool, err := pgxpool.New(startup, product.DatabaseURL)
		if err != nil {
			logger.Error("database configuration failed")
			return 1
		}
		defer pool.Close()
		if pool.Ping(startup) != nil {
			logger.Error("database unavailable during startup")
			return 1
		}
		if store.CheckSchema(startup, pool) != nil {
			logger.Error("product schema unavailable; run migrations and runtime grants")
			return 1
		}
		repository, err := store.New(pool)
		if err != nil {
			logger.Error("repository initialization failed")
			return 1
		}
		identity, err := auth.New(startup, product.Issuer, product.Audience)
		if err != nil {
			logger.Error("OIDC initialization failed")
			return 1
		}
		encoded, err := readPrivateFile(product.CursorKeyFile, 4096)
		if err != nil {
			logger.Error("cursor key file unavailable")
			return 1
		}
		key, err := base64.StdEncoding.Strict().DecodeString(strings.TrimSpace(string(encoded)))
		if err != nil || len(key) < 32 {
			logger.Error("cursor key must be base64 with at least 32 bytes")
			return 1
		}
		policyBytes, err := readPrivateFile(product.CorpusPolicyFile, 64*1024)
		if err != nil {
			logger.Error("corpus policy file unavailable")
			return 1
		}
		policy, err := httpapi.LoadCorpusPolicy(policyBytes)
		if err != nil {
			logger.Error("invalid corpus policy")
			return 1
		}
		handler, err = httpapi.NewRegistrationHandler(logger, readiness, httpapi.Registration{Repository: repository, Auth: identity, Pool: pool, Policy: policy, CursorKey: key, Timeout: product.Timeout})
		if err != nil {
			logger.Error("registration initialization failed")
			return 1
		}
	}
	server, err := service.New(c, logger, readiness, handler)
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

func readPrivateFile(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(b)) > limit {
		return nil, io.ErrUnexpectedEOF
	}
	return b, nil
}
