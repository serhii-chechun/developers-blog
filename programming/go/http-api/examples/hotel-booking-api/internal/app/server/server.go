package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

type (
	apiServer struct {
		storage
		workers

		handlerMux http.Handler
		config     config

		log *slog.Logger
	}
)

// New creates a new instance of apiServer with default configuration.
func New() *apiServer {
	return &apiServer{
		config: config{},
	}
}

// Run starts the API server and blocks until it is stopped or encounters an error.
func (s *apiServer) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.processConfig(ctx); err != nil {
		return fmt.Errorf("failed to process configuration: %w", err)
	}

	if err := s.init(); err != nil {
		return fmt.Errorf("failed to initialize API server: %w", err)
	}

	workersCtx, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()

	workersDone := make(chan struct{})
	go func() {
		defer close(workersDone)
		s.startWorkers(workersCtx)
	}()

	httpServer := &http.Server{
		Addr:    s.config.ListenAddress,
		Handler: s.handlerMux,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	failure := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			failure <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	var runErr error
	select {
	case err := <-failure:
		runErr = fmt.Errorf("API server error: %w", err)
	case <-quit:
		fmt.Println("shutting down API server...")

		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
		defer cancelShutdown()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			runErr = fmt.Errorf("API server shutdown error: %w", err)
			_ = httpServer.Close()
		}
	}

	stopWorkers()
	<-workersDone

	if err := s.storage.Close(); err != nil {
		runErr = errors.Join(runErr, fmt.Errorf("failed to close storage engine: %w", err))
	}

	return runErr
}
