package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type Client interface {
	Start(ctx context.Context) error
	Close(ctx context.Context) error
}

type Options struct {
	Token         string
	ClientID      string
	GuildID       string
	SyncCommands  bool
	Intents       discordgo.Intent
	Logger        *slog.Logger
	CommandRouter *CommandRouter
	MessageRouter *MessageRouter
}

type Gateway struct {
	session       gatewaySession
	logger        *slog.Logger
	clientID      string
	guildID       string
	syncCommands  bool
	commandRouter *CommandRouter
	messageRouter *MessageRouter
	started       bool
}

type gatewaySession interface {
	AddHandler(handler interface{}) func()
	ApplicationCommandBulkOverwrite(appID string, guildID string, commands []*discordgo.ApplicationCommand, options ...discordgo.RequestOption) ([]*discordgo.ApplicationCommand, error)
	Open() error
	Close() error
}

type sessionFactory func(token string, intents discordgo.Intent) (gatewaySession, error)

func NewGateway(opts Options) (*Gateway, error) {
	return newGatewayWithFactory(opts, discordgoSession)
}

func DefaultIntents() discordgo.Intent {
	return discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent
}

func newGatewayWithFactory(opts Options, factory sessionFactory) (*Gateway, error) {
	if strings.TrimSpace(opts.Token) == "" {
		return nil, fmt.Errorf("discord token is required")
	}
	if opts.Intents == 0 {
		opts.Intents = DefaultIntents()
	}
	if opts.SyncCommands && strings.TrimSpace(opts.ClientID) == "" {
		return nil, fmt.Errorf("discord client ID is required when command sync is enabled")
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	session, err := factory(botToken(opts.Token), opts.Intents)
	if err != nil {
		return nil, fmt.Errorf("create discord session: %w", err)
	}
	if opts.CommandRouter != nil {
		session.AddHandler(func(session *discordgo.Session, event *discordgo.InteractionCreate) {
			if err := opts.CommandRouter.HandleInteraction(context.Background(), session, event); err != nil {
				opts.Logger.Error("discord interaction failed", "error", err)
			}
		})
	}
	if opts.MessageRouter != nil {
		session.AddHandler(func(session *discordgo.Session, event *discordgo.MessageCreate) {
			if err := opts.MessageRouter.HandleMessage(context.Background(), session, event); err != nil {
				opts.Logger.Error("discord message failed", "error", err)
			}
		})
	}

	return &Gateway{
		session:       session,
		logger:        opts.Logger,
		clientID:      strings.TrimSpace(opts.ClientID),
		guildID:       strings.TrimSpace(opts.GuildID),
		syncCommands:  opts.SyncCommands,
		commandRouter: opts.CommandRouter,
		messageRouter: opts.MessageRouter,
	}, nil
}

func (g *Gateway) Start(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if g.started {
		return nil
	}
	if err := g.session.Open(); err != nil {
		return fmt.Errorf("open discord gateway: %w", err)
	}
	g.started = true
	if err := g.syncApplicationCommands(ctx); err != nil {
		if closeErr := g.Close(ctx); closeErr != nil {
			return fmt.Errorf("%w; close discord gateway: %v", err, closeErr)
		}
		return err
	}
	g.log().Info("discord gateway connected")
	return nil
}

func (g *Gateway) Close(ctx context.Context) error {
	if !g.started {
		return nil
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- g.session.Close()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("close discord gateway: %w", err)
		}
		g.started = false
		g.log().Info("discord gateway closed")
		return nil
	}
}

func (g *Gateway) log() *slog.Logger {
	if g.logger == nil {
		return slog.Default()
	}
	return g.logger
}

func (g *Gateway) syncApplicationCommands(ctx context.Context) error {
	if !g.syncCommands || g.commandRouter == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	commands := g.commandRouter.ApplicationCommands()
	if _, err := g.session.ApplicationCommandBulkOverwrite(g.clientID, g.guildID, commands); err != nil {
		return fmt.Errorf("sync discord application commands: %w", err)
	}
	g.log().Info("discord application commands synced", "count", len(commands), "guild_id", g.guildID)
	return nil
}

func discordgoSession(token string, intents discordgo.Intent) (gatewaySession, error) {
	session, err := discordgo.New(token)
	if err != nil {
		return nil, err
	}
	session.Identify.Intents = intents
	return session, nil
}

func botToken(token string) string {
	trimmed := strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(trimmed), "bot ") {
		return trimmed
	}
	return "Bot " + trimmed
}

type Interaction struct {
	GuildID          string
	ChannelID        string
	UserID           string
	RoleIDs          []string
	HasAdministrator bool
	Name             string
	Text             string
	Options          []InteractionOption
	Attachments      map[string]InteractionAttachment
	RoleService      GuildRoleService
}

type InteractionOption struct {
	Name    string
	Type    discordgo.ApplicationCommandOptionType
	Value   string
	Options []InteractionOption
}

type InteractionAttachment struct {
	ID          string
	URL         string
	Filename    string
	ContentType string
	Size        int
}
