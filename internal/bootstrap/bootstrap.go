package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"go-web-template/internal/config"
	"go-web-template/internal/httpapi"
	"go-web-template/internal/integration"
	"go-web-template/internal/note"
	"go-web-template/internal/platform/logging"
	"go-web-template/internal/storage"
	"go-web-template/internal/worker"
)

type Options struct {
	ConfigFile string
}

type application struct {
	logger       *logrus.Logger
	loggerCloser io.Closer
	storage      *storage.Storage
	integrations *integration.Integrations
	server       *http.Server
	workers      *worker.Group
	timeout      time.Duration
}

func Run(ctx context.Context, options Options) (err error) {
	app, err := newApplication(options)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, app.close()) }()

	listener, err := net.Listen("tcp", app.server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", app.server.Addr, err)
	}
	app.logger.WithField("address", listener.Addr().String()).Info("server listening")
	group, runCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		err := app.server.Serve(listener)
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	})
	group.Go(func() error { return app.workers.Run(runCtx) })
	group.Go(func() error {
		<-runCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), app.timeout)
		defer cancel()
		if err := app.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
		return nil
	})
	return group.Wait()
}

func newApplication(options Options) (*application, error) {
	cfg, err := config.Load(options.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}
	logger, loggerCloser, err := logging.New(cfg.Log.File, cfg.Log.Level)
	if err != nil {
		return nil, err
	}
	stores, err := storage.Open(cfg, logger)
	if err != nil {
		if loggerCloser != nil {
			_ = loggerCloser.Close()
		}
		return nil, err
	}
	integrations, err := integration.Open(cfg, logger)
	if err != nil {
		_ = stores.Close()
		if loggerCloser != nil {
			_ = loggerCloser.Close()
		}
		return nil, err
	}
	notes := note.NewService(note.NewRepository(stores.Primary.ORM()))
	readiness := append(stores.Checks(), integrations.Checks()...)
	server := &http.Server{
		Addr: cfg.HTTP.Addr,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			Logger: logger, Readiness: readiness, Notes: notes,
			HTTPPrefix: cfg.HTTP.Prefix, GinMode: cfg.HTTP.GinMode,
			CORSAllowedOrigins: cfg.CORS.AllowedOrigins, CORSAllowCredentials: cfg.CORS.AllowCredentials,
		}),
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
	}
	workers := worker.New()
	return &application{logger: logger, loggerCloser: loggerCloser, storage: stores, integrations: integrations, server: server, workers: workers, timeout: cfg.ShutdownTimeout}, nil
}

func (a *application) close() error {
	var errs []error
	if err := a.server.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		errs = append(errs, err)
	}
	if err := a.integrations.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := a.storage.Close(); err != nil {
		errs = append(errs, err)
	}
	if a.loggerCloser != nil {
		if err := a.loggerCloser.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
