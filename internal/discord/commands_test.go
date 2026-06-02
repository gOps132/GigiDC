package discord

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
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
