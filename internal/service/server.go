// Package service owns serving, process readiness and bounded shutdown.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/octieght18/forge/internal/config"
	"github.com/octieght18/forge/internal/httpapi"
)

type Server struct {
	config         config.Config
	logger         *slog.Logger
	readiness      *httpapi.Readiness
	http           *http.Server
	cancelRequests context.CancelFunc
}

// New injects the HTTP handler and logger; cmd/api is the composition root.
func New(c config.Config, logger *slog.Logger, readiness *httpapi.Readiness, handler http.Handler) (*Server, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if logger == nil || readiness == nil || handler == nil {
		return nil, fmt.Errorf("server dependencies must not be nil")
	}
	requestContext, cancel := context.WithCancel(context.Background())
	s := &Server{config: c, logger: logger, readiness: readiness, cancelRequests: cancel}
	s.http = &http.Server{
		Addr: c.Address, Handler: handler,
		ReadHeaderTimeout: c.ReadHeaderTimeout, ReadTimeout: c.ReadTimeout,
		WriteTimeout: c.WriteTimeout, IdleTimeout: c.IdleTimeout,
		MaxHeaderBytes: 1 << 20,
		BaseContext:    func(net.Listener) context.Context { return requestContext },
		ErrorLog:       slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
	return s, nil
}

// Run owns the listener and may be called once. Listener injection permits real
// network integration tests using OS-assigned ports without global configuration.
func (s *Server) Run(ctx context.Context, listener net.Listener) error {
	defer s.cancelRequests()
	defer s.readiness.Set(false)
	defer listener.Close()
	if ctx.Err() != nil {
		return nil
	}
	served := make(chan error, 1)
	s.readiness.Set(true)
	go func() { served <- s.http.Serve(listener) }()
	s.logger.Info("API listening", "address", listener.Addr().String())
	select {
	case err := <-served:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		// A failed accept loop must also release active connections.
		s.cancelRequests()
		_ = s.http.Close()
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		s.readiness.Set(false)
		s.logger.Info("API draining")
	}
	// Signal cancellation must not cancel requests before they have a chance to drain.
	shutdownContext, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()
	if err := s.http.Shutdown(shutdownContext); err != nil {
		s.cancelRequests()
		closeErr := s.http.Close()
		<-served
		return errors.Join(fmt.Errorf("graceful shutdown: %w", err), closeErr)
	}
	if err := <-served; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	s.logger.Info("API stopped")
	return nil
}
