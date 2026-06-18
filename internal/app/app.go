package app

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/gOps132/GigiDC/internal/assistant"
	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/buildinfo"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/config"
	"github.com/gOps132/GigiDC/internal/discord"
	"github.com/gOps132/GigiDC/internal/llm"
	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
	"github.com/gOps132/GigiDC/internal/memory"
	"github.com/gOps132/GigiDC/internal/plugins"
	"github.com/gOps132/GigiDC/internal/storage"
	"github.com/gOps132/GigiDC/internal/web"
)

type App struct {
	cfg           config.Config
	logger        *slog.Logger
	server        *http.Server
	readyCheck    web.ReadyCheck
	discordClient discord.Client
	db            *sql.DB
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
		var secretSealer llmprovider.SecretSealer
		llmSecretKey, err := cfg.DecodedLLMSecretKey()
		if err != nil {
			return nil, err
		}
		if llmSecretKey != nil {
			secretSealer, err = llmprovider.NewAESGCMSealer(llmSecretKey, cfg.LLMSecretKeyID)
			if err != nil {
				return nil, err
			}
		}

		dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		db, err := storage.OpenDB(dbCtx, cfg.DatabaseURL)
		if err != nil {
			return nil, err
		}
		application.db = db
		if err := storage.ApplyMigrationsFromDir(dbCtx, db, cfg.MigrationsDir); err != nil {
			_ = db.Close()
			return nil, err
		}

		grantStore := capability.NewSQLGrantStore(db)
		grantManager := capability.NewSQLGrantManager(db, func() string { return storage.NewID("capgrant") })
		auditStore := audit.NewStore(db, func() string { return storage.NewID("audit") })
		pluginStore := plugins.NewSQLCatalogStore(db, func() string { return storage.NewID("plugin") })
		providerStore := llmprovider.NewSQLStore(db, func() string { return storage.NewID("llm") })
		policyStore := llmprovider.NewSQLPolicyStore(db)
		providerService := llmprovider.NewServiceWithTester(providerStore, secretSealer, llmprovider.DefaultRegistry(), llmprovider.NewHTTPTester(nil))
		usageRecorder := llmprovider.NewSQLUsageRecorder(db, func() string { return storage.NewID("llmusage") })
		memoryStore := memory.NewSQLStore(db)
		memoryIngestor := memory.NewLiveIngestor(memoryStore, 512)
		conversationStore := assistant.NewSQLConversationStore(db, func() string { return storage.NewID("asstturn") })
		llmRuntime := llm.Runtime{
			Resolver:     providerService,
			Client:       llm.NewHTTPProviderClient(nil),
			Usage:        usageRecorder,
			NewRequestID: func() string { return storage.NewID("llmreq") },
		}
		assistantHandler := assistant.NewHandler(llmRuntime)
		assistantHandler.Recorder = conversationStore
		semanticPlanner := assistant.SemanticPluginPlanner{Runtime: llmRuntime}
		commands := discord.CoreCommands()
		commands = append(commands, discord.PermissionCommands(grantManager, nil, auditStore)...)
		commands = append(commands, discord.PluginCommands(pluginStore, plugins.HTTPManifestFetcher{}, auditStore)...)
		commands = append(commands, discord.LLMCommands(providerService, auditStore, discord.LLMCommandConfig{
			CredentialEntryEnabled: secretSealer != nil,
			UsageReporter:          usageRecorder,
			PolicyManager:          policyStore,
		})...)
		commands = append(commands, discord.MemoryCommands(memoryStore, auditStore)...)

		router, err := discord.NewCommandRouter(commands...)
		if err != nil {
			_ = db.Close()
			return nil, err
		}
		evaluator := capability.NewEvaluator(grantStore)
		router.SetAuthorizer(discord.NewCapabilityAuthorizer(evaluator, auditStore))

		externalAppHandler := discord.ExternalAppDryRunHandlerWithSemanticPolicy(
			pluginStore,
			evaluator,
			auditStore,
			discord.AssistantFallbackHandler(assistantHandler, discord.CoreMessageHandler()),
			semanticPlanner,
			policyStore,
		)
		semanticMemoryHandler := discord.SemanticMemoryHandler(
			memoryStore,
			evaluator,
			auditStore,
			policyStore,
			assistant.SemanticMemoryPlanner{Runtime: llmRuntime},
			externalAppHandler,
		)
		messageHandler := discord.MemoryQuestionHandler(memoryStore, evaluator, auditStore, semanticMemoryHandler)
		messageRouter, err := discord.NewMessageRouter(cfg.DiscordClientID, messageHandler, nil, memoryIngestor)
		if err != nil {
			_ = db.Close()
			return nil, err
		}
		client, err := discord.NewGateway(discord.Options{
			Token:         cfg.DiscordToken,
			ClientID:      cfg.DiscordClientID,
			GuildID:       cfg.DiscordGuildID,
			SyncCommands:  cfg.DiscordSyncCommands,
			Logger:        logger,
			CommandRouter: router,
			MessageRouter: messageRouter,
		})
		if err != nil {
			_ = db.Close()
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
		if closeErr := a.closeDatabase(); closeErr != nil {
			a.logger.Error("database shutdown after app error failed", "error", closeErr)
		}
		return err
	}
}

func (a *App) Shutdown(ctx context.Context) error {
	if err := a.closeDiscord(ctx); err != nil {
		return err
	}
	if err := a.closeDatabase(); err != nil {
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

func (a *App) closeDatabase() error {
	if a.db == nil {
		return nil
	}
	return a.db.Close()
}
