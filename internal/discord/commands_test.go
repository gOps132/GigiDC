package discord

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/gOps132/GigiDC/internal/capability"
)

func TestCommandRouterApplicationCommands(t *testing.T) {
	router, err := NewCommandRouter(CoreCommands()...)
	if err != nil {
		t.Fatalf("NewCommandRouter returned error: %v", err)
	}

	commands := router.ApplicationCommands()
	if len(commands) != 1 {
		t.Fatalf("commands = %d, want 1", len(commands))
	}
	if commands[0].Name != "ping" {
		t.Fatalf("command name = %q, want ping", commands[0].Name)
	}
}

func TestCommandRouterApplicationCommandsIncludeOptions(t *testing.T) {
	router, err := NewCommandRouter(Command{
		Name:        "admin",
		Description: "admin command",
		Options: []*discordgo.ApplicationCommandOption{{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "inspect",
			Description: "inspect things",
		}},
		Handle: okHandler,
	})
	if err != nil {
		t.Fatalf("NewCommandRouter returned error: %v", err)
	}

	commands := router.ApplicationCommands()
	if len(commands) != 1 || len(commands[0].Options) != 1 {
		t.Fatalf("commands = %+v, want one option", commands)
	}
	if commands[0].Options[0].Name != "inspect" {
		t.Fatalf("option name = %q, want inspect", commands[0].Options[0].Name)
	}
}

func TestCommandRouterRejectsInvalidCommands(t *testing.T) {
	tests := []struct {
		name    string
		command Command
	}{
		{name: "missing name", command: Command{Description: "desc", Handle: okHandler}},
		{name: "missing description", command: Command{Name: "ping", Handle: okHandler}},
		{name: "missing handler", command: Command{Name: "ping", Description: "desc"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCommandRouter(tt.command)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestCommandRouterRejectsDuplicateCommands(t *testing.T) {
	_, err := NewCommandRouter(
		Command{Name: "ping", Description: "one", Handle: okHandler},
		Command{Name: "ping", Description: "two", Handle: okHandler},
	)
	if err == nil {
		t.Fatal("expected duplicate command error")
	}
}

func TestCommandRouterHandlesPing(t *testing.T) {
	router, err := NewCommandRouter(CoreCommands()...)
	if err != nil {
		t.Fatalf("NewCommandRouter returned error: %v", err)
	}
	responder := &fakeResponder{}

	err = router.HandleInteraction(context.Background(), responder, applicationCommand("ping"))
	if err != nil {
		t.Fatalf("HandleInteraction returned error: %v", err)
	}
	if responder.content() != "pong" {
		t.Fatalf("response = %q, want pong", responder.content())
	}
}

func TestCommandRouterHandlesUnknownCommand(t *testing.T) {
	router, err := NewCommandRouter(CoreCommands()...)
	if err != nil {
		t.Fatalf("NewCommandRouter returned error: %v", err)
	}
	responder := &fakeResponder{}

	err = router.HandleInteraction(context.Background(), responder, applicationCommand("missing"))
	if err != nil {
		t.Fatalf("HandleInteraction returned error: %v", err)
	}
	if responder.content() != "Command not supported yet." {
		t.Fatalf("response = %q, want unsupported command message", responder.content())
	}
}

func TestCommandRouterHandlesCommandError(t *testing.T) {
	router, err := NewCommandRouter(Command{
		Name:        "fail",
		Description: "fail command",
		Handle: func(context.Context, Interaction) (CommandResponse, error) {
			return CommandResponse{}, errors.New("boom")
		},
	})
	if err != nil {
		t.Fatalf("NewCommandRouter returned error: %v", err)
	}
	responder := &fakeResponder{}

	err = router.HandleInteraction(context.Background(), responder, applicationCommand("fail"))
	if err != nil {
		t.Fatalf("HandleInteraction returned error: %v", err)
	}
	if responder.content() != "Command failed." {
		t.Fatalf("response = %q, want failed command message", responder.content())
	}
}

func TestCommandRouterIgnoresNilAndNonCommandInteractions(t *testing.T) {
	router, err := NewCommandRouter(CoreCommands()...)
	if err != nil {
		t.Fatalf("NewCommandRouter returned error: %v", err)
	}
	responder := &fakeResponder{}

	if err := router.HandleInteraction(context.Background(), responder, nil); err != nil {
		t.Fatalf("nil HandleInteraction returned error: %v", err)
	}
	if err := router.HandleInteraction(context.Background(), responder, &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{Type: discordgo.InteractionPing},
	}); err != nil {
		t.Fatalf("non-command HandleInteraction returned error: %v", err)
	}
	if responder.response != nil {
		t.Fatal("expected no response")
	}
}

func TestCommandRouterPassesInteractionContext(t *testing.T) {
	var got Interaction
	router, err := NewCommandRouter(Command{
		Name:        "inspect",
		Description: "inspect context",
		Handle: func(ctx context.Context, interaction Interaction) (CommandResponse, error) {
			got = interaction
			return CommandResponse{Content: "ok"}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewCommandRouter returned error: %v", err)
	}

	err = router.HandleInteraction(context.Background(), &fakeResponder{}, &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type:      discordgo.InteractionApplicationCommand,
			GuildID:   "guild-id",
			ChannelID: "channel-id",
			Member:    &discordgo.Member{User: &discordgo.User{ID: "user-id"}},
			Data:      discordgo.ApplicationCommandInteractionData{Name: "inspect"},
		},
	})
	if err != nil {
		t.Fatalf("HandleInteraction returned error: %v", err)
	}
	if got.GuildID != "guild-id" || got.ChannelID != "channel-id" || got.UserID != "user-id" || got.Name != "inspect" {
		t.Fatalf("interaction context = %+v", got)
	}
}

func TestCommandRouterPassesNestedOptions(t *testing.T) {
	var got Interaction
	router, err := NewCommandRouter(Command{
		Name:        "permissions",
		Description: "permissions command",
		Handle: func(ctx context.Context, interaction Interaction) (CommandResponse, error) {
			got = interaction
			return CommandResponse{Content: "ok"}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewCommandRouter returned error: %v", err)
	}

	err = router.HandleInteraction(context.Background(), &fakeResponder{}, &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type:    discordgo.InteractionApplicationCommand,
			GuildID: "guild-id",
			Member:  &discordgo.Member{User: &discordgo.User{ID: "user-id"}},
			Data: discordgo.ApplicationCommandInteractionData{
				Name: "permissions",
				Options: []*discordgo.ApplicationCommandInteractionDataOption{{
					Name: "grant-role",
					Type: discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{Name: "role", Type: discordgo.ApplicationCommandOptionRole, Value: "role-id"},
						{Name: "capability", Type: discordgo.ApplicationCommandOptionString, Value: "plugin.install"},
					},
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("HandleInteraction returned error: %v", err)
	}
	if len(got.Options) != 1 || got.Options[0].Name != "grant-role" || len(got.Options[0].Options) != 2 {
		t.Fatalf("options = %+v, want nested subcommand options", got.Options)
	}
	if got.Options[0].Options[1].Value != "plugin.install" {
		t.Fatalf("capability option = %+v, want plugin.install", got.Options[0].Options[1])
	}
}

func TestCommandRouterDeniesMissingCapability(t *testing.T) {
	calls := 0
	router, err := NewCommandRouter(Command{
		Name:               "admin",
		Description:        "admin command",
		RequiredCapability: "job.admin",
		Handle: func(context.Context, Interaction) (CommandResponse, error) {
			calls++
			return CommandResponse{Content: "unexpected"}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewCommandRouter returned error: %v", err)
	}
	router.SetAuthorizer(fakeCommandAuthorizer{decision: capability.Decision{
		Allowed: false,
		Reason:  capability.ReasonMissingCapability,
	}})
	responder := &fakeResponder{}

	err = router.HandleInteraction(context.Background(), responder, applicationCommand("admin"))
	if err != nil {
		t.Fatalf("HandleInteraction returned error: %v", err)
	}
	if calls != 0 {
		t.Fatalf("handler calls = %d, want 0", calls)
	}
	if responder.content() != "Permission denied." {
		t.Fatalf("response = %q, want permission denied", responder.content())
	}
	if responder.flags() != discordgo.MessageFlagsEphemeral {
		t.Fatalf("flags = %v, want ephemeral", responder.flags())
	}
}

func TestCommandRouterAllowsCapability(t *testing.T) {
	calls := 0
	var gotRequired capability.Capability
	router, err := NewCommandRouter(Command{
		Name:               "admin",
		Description:        "admin command",
		RequiredCapability: "job.admin",
		Handle: func(context.Context, Interaction) (CommandResponse, error) {
			calls++
			return CommandResponse{Content: "allowed"}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewCommandRouter returned error: %v", err)
	}
	router.SetAuthorizer(fakeCommandAuthorizer{
		decision: capability.Decision{Allowed: true, Reason: capability.ReasonUserGrant},
		capability: func(required capability.Capability) {
			gotRequired = required
		},
	})
	responder := &fakeResponder{}

	err = router.HandleInteraction(context.Background(), responder, applicationCommand("admin"))
	if err != nil {
		t.Fatalf("HandleInteraction returned error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("handler calls = %d, want 1", calls)
	}
	if gotRequired != "job.admin" {
		t.Fatalf("required capability = %q, want job.admin", gotRequired)
	}
	if responder.content() != "allowed" {
		t.Fatalf("response = %q, want allowed", responder.content())
	}
}

func TestCommandRouterPassesRolesAndAdministratorFlag(t *testing.T) {
	var got Interaction
	router, err := NewCommandRouter(Command{
		Name:        "inspect",
		Description: "inspect context",
		Handle: func(ctx context.Context, interaction Interaction) (CommandResponse, error) {
			got = interaction
			return CommandResponse{Content: "ok"}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewCommandRouter returned error: %v", err)
	}

	err = router.HandleInteraction(context.Background(), &fakeResponder{}, &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type:      discordgo.InteractionApplicationCommand,
			GuildID:   "guild-id",
			ChannelID: "channel-id",
			Member: &discordgo.Member{
				User:        &discordgo.User{ID: "user-id"},
				Roles:       []string{"role-1", "role-2"},
				Permissions: discordgo.PermissionAdministrator,
			},
			Data: discordgo.ApplicationCommandInteractionData{Name: "inspect"},
		},
	})
	if err != nil {
		t.Fatalf("HandleInteraction returned error: %v", err)
	}
	if len(got.RoleIDs) != 2 || !got.HasAdministrator {
		t.Fatalf("interaction context = %+v, want roles and admin flag", got)
	}
}

func okHandler(context.Context, Interaction) (CommandResponse, error) {
	return CommandResponse{Content: "ok"}, nil
}

func applicationCommand(name string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type: discordgo.InteractionApplicationCommand,
			Data: discordgo.ApplicationCommandInteractionData{Name: name},
		},
	}
}

type fakeResponder struct {
	response *discordgo.InteractionResponse
}

func (r *fakeResponder) InteractionRespond(_ *discordgo.Interaction, response *discordgo.InteractionResponse, _ ...discordgo.RequestOption) error {
	r.response = response
	return nil
}

func (r *fakeResponder) content() string {
	if r.response == nil || r.response.Data == nil {
		return ""
	}
	return r.response.Data.Content
}

func (r *fakeResponder) flags() discordgo.MessageFlags {
	if r.response == nil || r.response.Data == nil {
		return 0
	}
	return r.response.Data.Flags
}

type fakeCommandAuthorizer struct {
	decision   capability.Decision
	err        error
	capability func(capability.Capability)
}

func (a fakeCommandAuthorizer) Check(ctx context.Context, interaction Interaction, required capability.Capability) (capability.Decision, error) {
	if a.capability != nil {
		a.capability(required)
	}
	return a.decision, a.err
}
