package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gOps132/GigiDC/internal/buildinfo"
	"github.com/gOps132/GigiDC/internal/config"
	"github.com/gOps132/GigiDC/internal/storage"
	"github.com/gOps132/GigiDC/internal/web"
)

type App struct {
	cfg        config.Config
	logger     *slog.Logger
	server     *http.Server
	readyCheck web.ReadyCheck
}

func New(cfg config.Config, logger *slog.Logger) *App {
	checker := storage.NewTCPReadyCheck(cfg.DatabaseURL, 2*time.Second)
	return &App{
		cfg:        cfg,
		logger:     logger,
		readyCheck: checker.Ready,
	}
}

func (a *App) Run(ctx context.Context) error {
	mux := web.NewServer(web.Options{
		Build: buildinfo.Current(),
		Ready: a.readyCheck,
	})

	a.server = &http.Server{
		Addr:              a.cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("gigi http server listening", "addr", a.cfg.HTTPAddr, "env", a.cfg.Env)
		errCh <- a.server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func (a *App) Shutdown(ctx context.Context) error {
	if a.server == nil {
		return nil
	}
	return a.server.Shutdown(ctx)
}
