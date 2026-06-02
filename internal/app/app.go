package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gOps132/GigiDC/internal/buildinfo"
	"github.com/gOps132/GigiDC/internal/config"
	"github.com/gOps132/GigiDC/internal/discord"
	"github.com/gOps132/GigiDC/internal/storage"
	"github.com/gOps132/GigiDC/internal/web"
)

type App struct {
	cfg           config.Config
	logger        *slog.Logger
	server        *http.Server
	readyCheck    web.ReadyCheck
	discordClient discord.Client
}

type Option func(*App)

func WithReadyCheck(check web.ReadyCheck) Option {
	return func(a *App) {
		a.readyCheck = check
	}
}

func WithDiscordClient(client discord.Client) Option {
	return func(a *App) {
		a.discordClient = client
	}
}

func New(cfg config.Config, logger *slog.Logger, opts ...Option) (*App, error) {
	checker := storage.NewTCPReadyCheck(cfg.DatabaseURL, 2*time.Second)
	application := &App{
		cfg:        cfg,
		logger:     logger,
		readyCheck: checker.Ready,
	}

	for _, opt := range opts {
		opt(application)
	}

	if cfg.DiscordEnabled && application.discordClient == nil {
		router, err := discord.NewCommandRouter(discord.CoreCommands()...)
		if err != nil {
			return nil, err
		}
		client, err := discord.NewGateway(discord.Options{
			Token:         cfg.DiscordToken,
			ClientID:      cfg.DiscordClientID,
			Logger:        logger,
			CommandRouter: router,
		})
		if err != nil {
			return nil, err
		}
		application.discordClient = client
	}

	return application, nil
}

func (a *App) Run(ctx context.Context) error {
	if a.discordClient != nil {
		if err := a.discordClient.Start(ctx); err != nil {
			return err
		}
	}

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
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if closeErr := a.closeDiscord(shutdownCtx); closeErr != nil {
			a.logger.Error("discord shutdown after app error failed", "error", closeErr)
		}
		return err
	}
}

func (a *App) Shutdown(ctx context.Context) error {
	if err := a.closeDiscord(ctx); err != nil {
		return err
	}
	if a.server == nil {
		return nil
	}
	return a.server.Shutdown(ctx)
}

func (a *App) closeDiscord(ctx context.Context) error {
	if a.discordClient == nil {
		return nil
	}
	return a.discordClient.Close(ctx)
}
