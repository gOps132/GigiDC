package discord

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
)

func TestPermissionCommandsExposeSubcommands(t *testing.T) {
	commands := PermissionCommands(&fakeGrantManager{}, nil)
	if len(commands) != 1 {
		t.Fatalf("commands = %d, want 1", len(commands))
	}
	command := commands[0]
	if command.RequiredCapability != capability.CapabilityManage {
		t.Fatalf("required capability = %q, want capability.manage", command.RequiredCapability)
	}
	if len(command.Options) != 4 {
		t.Fatalf("options = %d, want 4", len(command.Options))
	}
	if command.Options[0].Options[0].Type != discordgo.ApplicationCommandOptionRole {
		t.Fatalf("grant-role target type = %v, want role", command.Options[0].Options[0].Type)
	}
}

func TestPermissionCommandGrantsRole(t *testing.T) {
	manager := &fakeGrantManager{}
	recorder := &fakeAuditRecorder{}
	handler := PermissionCommands(manager, recorder)[0].Handle

	response, err := handler(context.Background(), Interaction{
		GuildID: "guild-id",
		UserID:  "actor-id",
		Name:    "permissions",
		Options: []InteractionOption{{
			Name: "grant-role",
			Options: []InteractionOption{
				{Name: "role", Value: "role-id"},
				{Name: "capability", Value: "plugin.install"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if manager.method != "GrantRole" || manager.guildID != "guild-id" || manager.targetID != "role-id" || manager.capability != "plugin.install" || manager.actorID != "actor-id" {
		t.Fatalf("manager call = %+v", manager)
	}
	if !strings.Contains(response.Content, "Granted `plugin.install`") {
		t.Fatalf("response = %q, want grant success", response.Content)
	}
	if !response.Ephemeral {
		t.Fatal("response Ephemeral = false, want true")
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusSucceeded {
		t.Fatalf("audit events = %+v, want succeeded", recorder.events)
	}
}

func TestPermissionCommandRevokesUser(t *testing.T) {
	manager := &fakeGrantManager{}
	handler := PermissionCommands(manager, nil)[0].Handle

	response, err := handler(context.Background(), Interaction{
		GuildID: "guild-id",
		UserID:  "actor-id",
		Name:    "permissions",
		Options: []InteractionOption{{
			Name: "revoke-user",
			Options: []InteractionOption{
				{Name: "user", Value: "user-id"},
				{Name: "capability", Value: "job.admin"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if manager.method != "RevokeUser" || manager.targetID != "user-id" || manager.capability != "job.admin" {
		t.Fatalf("manager call = %+v", manager)
	}
	if !strings.Contains(response.Content, "Revoked `job.admin`") {
		t.Fatalf("response = %q, want revoke success", response.Content)
	}
}

func TestPermissionCommandRejectsDMAndInvalidCapability(t *testing.T) {
	handler := PermissionCommands(&fakeGrantManager{}, nil)[0].Handle

	response, err := handler(context.Background(), Interaction{
		UserID: "actor-id",
		Name:   "permissions",
		Options: []InteractionOption{{
			Name: "grant-role",
			Options: []InteractionOption{
				{Name: "role", Value: "role-id"},
				{Name: "capability", Value: "Plugin Install"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !strings.Contains(response.Content, "inside a Discord server") {
		t.Fatalf("response = %q, want guild-only message", response.Content)
	}
	if !response.Ephemeral {
		t.Fatal("response Ephemeral = false, want true")
	}
}

func TestPermissionCommandAuditsManagerFailure(t *testing.T) {
	manager := &fakeGrantManager{err: errors.New("write failed")}
	recorder := &fakeAuditRecorder{}
	handler := PermissionCommands(manager, recorder)[0].Handle

	_, err := handler(context.Background(), Interaction{
		GuildID: "guild-id",
		UserID:  "actor-id",
		Name:    "permissions",
		Options: []InteractionOption{{
			Name: "grant-user",
			Options: []InteractionOption{
				{Name: "user", Value: "user-id"},
				{Name: "capability", Value: "job.admin"},
			},
		}},
	})
	if err == nil {
		t.Fatal("expected manager error")
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusFailed {
		t.Fatalf("audit events = %+v, want failed", recorder.events)
	}
}

type fakeGrantManager struct {
	method     string
	guildID    string
	targetID   string
	capability capability.Capability
	actorID    string
	err        error
}

func (m *fakeGrantManager) GrantRole(_ context.Context, guildID string, roleID string, cap capability.Capability, actorID string) error {
	m.method, m.guildID, m.targetID, m.capability, m.actorID = "GrantRole", guildID, roleID, cap, actorID
	return m.err
}

func (m *fakeGrantManager) RevokeRole(_ context.Context, guildID string, roleID string, cap capability.Capability) error {
	m.method, m.guildID, m.targetID, m.capability = "RevokeRole", guildID, roleID, cap
	return m.err
}

func (m *fakeGrantManager) GrantUser(_ context.Context, guildID string, userID string, cap capability.Capability, actorID string) error {
	m.method, m.guildID, m.targetID, m.capability, m.actorID = "GrantUser", guildID, userID, cap, actorID
	return m.err
}

func (m *fakeGrantManager) RevokeUser(_ context.Context, guildID string, userID string, cap capability.Capability) error {
	m.method, m.guildID, m.targetID, m.capability = "RevokeUser", guildID, userID, cap
	return m.err
}

type fakeAuditRecorder struct {
	events []audit.Event
	err    error
}

func (r *fakeAuditRecorder) Record(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return r.err
}
