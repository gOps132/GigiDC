package discord

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/memory"
)

func TestMemoryCommandsExposeStatusAndSettingsSurface(t *testing.T) {
	commands := MemoryCommands(&fakeMemoryManager{}, nil)
	if len(commands) != 1 || commands[0].Name != "memory" {
		t.Fatalf("commands = %+v, want memory command", commands)
	}
	command := commands[0]
	if findOption(command.Options, "status") == nil {
		t.Fatal("memory command missing status")
	}
	count := findOption(command.Options, "count")
	if count == nil {
		t.Fatal("memory command missing count")
	}
	if option := findOption(count.Options, "text"); option == nil || option.Type != discordgo.ApplicationCommandOptionString || !option.Required {
		t.Fatalf("count text option = %+v, want required string", option)
	}
	settings := findOption(command.Options, "settings")
	if settings == nil || settings.Type != discordgo.ApplicationCommandOptionSubCommandGroup {
		t.Fatalf("settings = %+v, want subcommand group", settings)
	}
	for _, name := range []string{"show", "set"} {
		if findOption(settings.Options, name) == nil {
			t.Fatalf("settings missing %q", name)
		}
	}
	set := findOption(settings.Options, "set")
	if option := findOption(set.Options, "channel"); option == nil || option.Type != discordgo.ApplicationCommandOptionChannel || !option.Required {
		t.Fatalf("channel option = %+v, want required channel", option)
	}
	if option := findOption(set.Options, "mode"); option == nil || !hasChoice(option, "off") || !hasChoice(option, "metadata") || !hasChoice(option, "full") {
		t.Fatalf("mode option = %+v, want mode choices", option)
	}
	if option := findOption(set.Options, "retention-days"); option == nil || option.Type != discordgo.ApplicationCommandOptionInteger || option.Required {
		t.Fatalf("retention-days option = %+v, want optional integer", option)
	}
}

func TestMemoryCommandDynamicCapabilities(t *testing.T) {
	tests := []struct {
		name string
		i    Interaction
		want capability.Capability
	}{
		{name: "status", i: memoryStatusInteraction(), want: "memory.read.guild"},
		{name: "count", i: memoryCountInteraction([]InteractionOption{{Name: "text", Value: "postgres"}}), want: "memory.read.guild"},
		{name: "settings show", i: memorySettingsInteraction("show", nil), want: "memory.manage.guild"},
		{name: "settings set", i: memorySettingsInteraction("set", []InteractionOption{{Name: "channel", Value: "channel-id"}, {Name: "mode", Value: "metadata"}}), want: "memory.manage.guild"},
		{name: "bad path", i: memorySettingsInteraction("wat", nil), want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MemoryCommands(&fakeMemoryManager{}, nil)[0].RequiredCapabilityFor(tt.i)
			if got != tt.want {
				t.Fatalf("capability = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMemoryCountReturnsExactMentionCount(t *testing.T) {
	manager := &fakeMemoryManager{count: memory.CountResult{Count: 3}}
	handler := MemoryCommands(manager, nil)[0].Handle

	response, err := handler(context.Background(), memoryCountInteraction([]InteractionOption{
		{Name: "text", Value: "Postgres"},
		{Name: "user", Value: "user-id"},
	}))
	if err != nil {
		t.Fatalf("count returned error: %v", err)
	}
	if manager.method != "CountMentions" || manager.countReq.GuildID != "guild-id" || manager.countReq.ChannelID != "channel-id" || manager.countReq.AuthorUserID != "user-id" || manager.countReq.Text != "Postgres" {
		t.Fatalf("manager = %+v, want count request", manager)
	}
	if response.Content != "<@user-id> mentioned `postgres` 3 times in this channel." || !response.Ephemeral {
		t.Fatalf("response = %+v, want count response", response)
	}
}

func TestMemoryCommandRejectsDMs(t *testing.T) {
	handler := MemoryCommands(&fakeMemoryManager{}, nil)[0].Handle

	response, err := handler(context.Background(), Interaction{Name: "memory", UserID: "actor-id", Options: []InteractionOption{{Name: "status"}}})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if response.Content != "Memory can only be managed inside a Discord server." || !response.Ephemeral {
		t.Fatalf("response = %+v, want guild-only error", response)
	}
}

func TestMemoryStatusReturnsPolicyAndChannels(t *testing.T) {
	manager := &fakeMemoryManager{status: memory.Status{
		Policy: memory.Policy{GuildID: "guild-id", DefaultRetentionDays: 90, RawStorageMode: memory.ModeMetadata},
		Channels: []memory.ChannelPolicy{
			{GuildID: "guild-id", ChannelID: "channel-id", Mode: memory.ModeFull, RetentionDays: 30},
		},
	}}
	handler := MemoryCommands(manager, nil)[0].Handle

	response, err := handler(context.Background(), memoryStatusInteraction())
	if err != nil {
		t.Fatalf("status returned error: %v", err)
	}
	if manager.method != "GuildStatus" || manager.guildID != "guild-id" {
		t.Fatalf("manager = %+v, want guild status", manager)
	}
	if !strings.Contains(response.Content, "Guild memory: default `metadata`, retention 90 days") || !strings.Contains(response.Content, "<#channel-id>") || !response.Ephemeral {
		t.Fatalf("response = %+v, want memory status", response)
	}
}

func TestMemorySettingsSetStoresChannelPolicyAndAudits(t *testing.T) {
	manager := &fakeMemoryManager{}
	recorder := &fakeAuditRecorder{}
	handler := MemoryCommands(manager, recorder)[0].Handle

	response, err := handler(context.Background(), memorySettingsInteraction("set", []InteractionOption{
		{Name: "channel", Value: "channel-id"},
		{Name: "mode", Value: "full"},
		{Name: "retention-days", Value: "30"},
	}))
	if err != nil {
		t.Fatalf("set returned error: %v", err)
	}
	if manager.method != "UpsertChannelPolicy" || manager.upsertReq.GuildID != "guild-id" || manager.upsertReq.ChannelID != "channel-id" || manager.upsertReq.Mode != memory.ModeFull || manager.upsertReq.RetentionDays != 30 || manager.upsertReq.ActorID != "actor-id" {
		t.Fatalf("manager = %+v, want upsert request", manager)
	}
	if response.Content != "Set memory for <#channel-id> to `full` (retention: 30 days)." || !response.Ephemeral {
		t.Fatalf("response = %+v, want set response", response)
	}
	if len(recorder.events) != 1 || recorder.events[0].Kind != "discord.memory.settings.change" || recorder.events[0].Status != audit.StatusSucceeded || recorder.events[0].Metadata["channel_id"] != "channel-id" || recorder.events[0].Metadata["mode"] != "full" {
		t.Fatalf("audit events = %+v, want successful memory audit", recorder.events)
	}
}

func TestMemorySettingsSetValidatesRetention(t *testing.T) {
	handler := MemoryCommands(&fakeMemoryManager{}, nil)[0].Handle

	response, err := handler(context.Background(), memorySettingsInteraction("set", []InteractionOption{
		{Name: "channel", Value: "channel-id"},
		{Name: "mode", Value: "metadata"},
		{Name: "retention-days", Value: "366"},
	}))
	if err != nil {
		t.Fatalf("set returned error: %v", err)
	}
	if response.Content != "Retention days must be between 1 and 365." || !response.Ephemeral {
		t.Fatalf("response = %+v, want validation error", response)
	}
}

func TestMemoryCommandFailureIsCleanAndAudited(t *testing.T) {
	recorder := &fakeAuditRecorder{}
	handler := MemoryCommands(&fakeMemoryManager{err: errors.New("db down with message text")}, recorder)[0].Handle

	response, err := handler(context.Background(), memorySettingsInteraction("set", []InteractionOption{
		{Name: "channel", Value: "channel-id"},
		{Name: "mode", Value: "metadata"},
	}))
	if err != nil {
		t.Fatalf("set returned error: %v", err)
	}
	if response.Content != "Memory command failed." || strings.Contains(response.Content, "db down") || !response.Ephemeral {
		t.Fatalf("response = %+v, want clean failure", response)
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusFailed || recorder.events[0].Reason != "memory_action_failed" {
		t.Fatalf("audit events = %+v, want failed memory audit", recorder.events)
	}
}

func memoryStatusInteraction() Interaction {
	return Interaction{
		GuildID: "guild-id",
		UserID:  "actor-id",
		Name:    "memory",
		Options: []InteractionOption{{Name: "status"}},
	}
}

func memorySettingsInteraction(action string, options []InteractionOption) Interaction {
	return Interaction{
		GuildID: "guild-id",
		UserID:  "actor-id",
		Name:    "memory",
		Options: []InteractionOption{{
			Name: "settings",
			Options: []InteractionOption{{
				Name:    action,
				Options: options,
			}},
		}},
	}
}

func memoryCountInteraction(options []InteractionOption) Interaction {
	return Interaction{
		GuildID:   "guild-id",
		ChannelID: "channel-id",
		UserID:    "actor-id",
		Name:      "memory",
		Options: []InteractionOption{{
			Name:    "count",
			Options: options,
		}},
	}
}

type fakeMemoryManager struct {
	method    string
	guildID   string
	status    memory.Status
	upsertReq memory.UpsertChannelPolicyRequest
	countReq  memory.CountRequest
	count     memory.CountResult
	err       error
}

func (m *fakeMemoryManager) GuildStatus(ctx context.Context, guildID string) (memory.Status, error) {
	m.method = "GuildStatus"
	m.guildID = guildID
	if m.err != nil {
		return memory.Status{}, m.err
	}
	if m.status.Policy.GuildID == "" {
		return memory.Status{Policy: memory.DefaultPolicy(guildID)}, nil
	}
	return m.status, nil
}

func (m *fakeMemoryManager) UpsertChannelPolicy(ctx context.Context, req memory.UpsertChannelPolicyRequest) (memory.ChannelPolicy, error) {
	m.method = "UpsertChannelPolicy"
	m.upsertReq = req
	if m.err != nil {
		return memory.ChannelPolicy{}, m.err
	}
	return memory.ChannelPolicy{
		GuildID:         req.GuildID,
		ChannelID:       req.ChannelID,
		Mode:            req.Mode,
		RetentionDays:   req.RetentionDays,
		UpdatedByUserID: req.ActorID,
	}, nil
}

func (m *fakeMemoryManager) CountMentions(ctx context.Context, req memory.CountRequest) (memory.CountResult, error) {
	m.method = "CountMentions"
	m.countReq = req
	return m.count, m.err
}
