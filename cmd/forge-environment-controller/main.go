package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/octieght18/forge/internal/environment"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	ctrl.SetLogger(logr.FromSlogHandler(logger.Handler()))
	if err := run(); err != nil {
		slog.Error("Environment controller stopped", "reason", "startup_or_runtime_failure")
		os.Exit(1)
	}
}
func run() error {
	ctx := ctrl.SetupSignalHandler()
	dsn := os.Getenv("FORGE_ENVIRONMENT_DATABASE_URL")
	if dsn == "" {
		return errors.New("environment database configuration required")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return err
	}
	cfg.MaxConns = 4
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	cfg.MaxConnLifetime = 5 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	scheme := runtime.NewScheme()
	if err := core.AddToScheme(scheme); err != nil {
		return err
	}
	kube, err := ctrl.GetConfig()
	if err != nil {
		return err
	}
	m, err := ctrl.NewManager(kube, ctrl.Options{Scheme: scheme, Metrics: metrics.Options{BindAddress: "0"}, HealthProbeBindAddress: ":8084", LeaderElection: true, LeaderElectionID: "forge-environment-controller", LeaderElectionNamespace: "forge-local", GracefulShutdownTimeout: duration(10 * time.Second)})
	if err != nil {
		return err
	}
	// Direct reads avoid cache lag when checking ownership before mutation/deletion.
	c, err := client.New(m.GetConfig(), client.Options{Scheme: scheme})
	if err != nil {
		return err
	}
	r := &environment.Reconciler{Client: c, Lookup: environment.Database{Pool: pool}}
	if err := r.Setup(m); err != nil {
		return err
	}
	if err := m.AddHealthzCheck("process", healthz.Ping); err != nil {
		return err
	}
	if err := m.AddReadyzCheck("database", func(_ *http.Request) error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		select {
		case <-m.Elected():
		default:
			return errors.New("controller is not leader")
		}
		if !m.GetCache().WaitForCacheSync(ctx) {
			return errors.New("controller cache not synchronized")
		}
		return pool.Ping(ctx)
	}); err != nil {
		return err
	}
	return m.Start(ctx)
}
func duration(d time.Duration) *time.Duration { return &d }
