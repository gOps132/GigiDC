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

func TestPermissionCommandsExposeRoleFirstGroups(t *testing.T) {
	commands := PermissionCommands(&fakeGrantManager{}, &fakeGuildRoleService{}, nil)
	if len(commands) != 1 {
		t.Fatalf("commands = %d, want 1", len(commands))
	}
	command := commands[0]
	if command.RequiredCapability != capability.CapabilityManage {
		t.Fatalf("required capability = %q, want capability.manage", command.RequiredCapability)
	}
	roleGroup := findOption(command.Options, "role")
	if roleGroup == nil || roleGroup.Type != discordgo.ApplicationCommandOptionSubCommandGroup {
		t.Fatalf("role option = %+v, want subcommand group", roleGroup)
	}
	for _, name := range []string{"create", "assign", "unassign", "grant", "revoke", "grant-preset", "revoke-preset"} {
		if findOption(roleGroup.Options, name) == nil {
			t.Fatalf("role group missing %q", name)
		}
	}
	userGroup := findOption(command.Options, "user")
	if userGroup == nil || userGroup.Type != discordgo.ApplicationCommandOptionSubCommandGroup {
		t.Fatalf("user option = %+v, want subcommand group", userGroup)
	}
	if findOption(userGroup.Options, "grant") == nil || findOption(userGroup.Options, "revoke") == nil {
		t.Fatalf("user group options = %+v, want grant and revoke", userGroup.Options)
	}
}

func TestPermissionCommandUsesCapabilityAndPresetChoices(t *testing.T) {
	commands := PermissionCommands(&fakeGrantManager{}, &fakeGuildRoleService{}, nil)
	roleGroup := findOption(commands[0].Options, "role")
	grant := findOption(roleGroup.Options, "grant")
	capabilityOption := findOption(grant.Options, "capability")
	if !hasChoice(capabilityOption, "plugin.install") || !hasChoice(capabilityOption, "capability.manage") || !hasChoice(capabilityOption, "memory.manage.guild") {
		t.Fatalf("capability choices = %+v, want known capabilities", capabilityOption.Choices)
	}
	create := findOption(roleGroup.Options, "create")
	presetOption := findOption(create.Options, "preset")
	if !hasChoice(presetOption, "gigi-admin") || !hasChoice(presetOption, "plugin-manager") || !hasChoice(presetOption, "memory-manager") {
		t.Fatalf("preset choices = %+v, want known presets", presetOption.Choices)
	}
}

func TestPermissionCommandCreatesRoleAndGrantsPreset(t *testing.T) {
	manager := &fakeGrantManager{}
	roles := &fakeGuildRoleService{createdRoleID: "role-id"}
	recorder := &fakeAuditRecorder{}
	handler := PermissionCommands(manager, roles, recorder)[0].Handle

	response, err := handler(context.Background(), Interaction{
		GuildID:     "guild-id",
		UserID:      "actor-id",
		Name:        "permissions",
		RoleService: roles,
		Options: []InteractionOption{{
			Name: "role",
			Options: []InteractionOption{{
				Name: "create",
				Options: []InteractionOption{
					{Name: "name", Value: "Gigi Plugin Managers"},
					{Name: "preset", Value: "plugin-manager"},
				},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if roles.createdName != "Gigi Plugin Managers" {
		t.Fatalf("created role name = %q, want Gigi Plugin Managers", roles.createdName)
	}
	if manager.method != "GrantRoleCapabilities" || manager.targetID != "role-id" || len(manager.capabilities) != 1 || manager.capabilities[0] != "plugin.install" {
		t.Fatalf("manager call = %+v, want plugin-manager preset grant", manager)
	}
	if !strings.Contains(response.Content, "Created role") || !response.Ephemeral {
		t.Fatalf("response = %+v, want ephemeral create success", response)
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusSucceeded {
		t.Fatalf("audit events = %+v, want succeeded", recorder.events)
	}
}

func TestPermissionCommandAssignsAndUnassignsRole(t *testing.T) {
	roles := &fakeGuildRoleService{}
	handler := PermissionCommands(&fakeGrantManager{}, roles, nil)[0].Handle

	assignResponse, err := handler(context.Background(), roleInteraction("assign", "role-id", "user-id"))
	if err != nil {
		t.Fatalf("assign returned error: %v", err)
	}
	if roles.addedRoleID != "role-id" || roles.addedUserID != "user-id" {
		t.Fatalf("added role/user = %q/%q, want role-id/user-id", roles.addedRoleID, roles.addedUserID)
	}
	if !strings.Contains(assignResponse.Content, "<@&role-id>") || !strings.Contains(assignResponse.Content, "<@user-id>") || !assignResponse.Ephemeral {
		t.Fatalf("assign response = %+v, want role/user mentions", assignResponse)
	}

	unassignResponse, err := handler(context.Background(), roleInteraction("unassign", "role-id", "user-id"))
	if err != nil {
		t.Fatalf("unassign returned error: %v", err)
	}
	if roles.removedRoleID != "role-id" || roles.removedUserID != "user-id" {
		t.Fatalf("removed role/user = %q/%q, want role-id/user-id", roles.removedRoleID, roles.removedUserID)
	}
	if !strings.Contains(unassignResponse.Content, "<@&role-id>") || !strings.Contains(unassignResponse.Content, "<@user-id>") || !unassignResponse.Ephemeral {
		t.Fatalf("unassign response = %+v, want role/user mentions", unassignResponse)
	}
}

func TestPermissionCommandRoleServiceFailureIsClean(t *testing.T) {
	roles := &fakeGuildRoleService{err: errors.New("Missing Permissions")}
	handler := PermissionCommands(&fakeGrantManager{}, roles, nil)[0].Handle

	response, err := handler(context.Background(), roleInteraction("assign", "role-id", "user-id"))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !strings.Contains(response.Content, "Manage Roles") || !response.Ephemeral {
		t.Fatalf("response = %+v, want clean role permission error", response)
	}
}

func TestPermissionCommandGrantsAndRevokesRolePreset(t *testing.T) {
	manager := &fakeGrantManager{}
	handler := PermissionCommands(manager, &fakeGuildRoleService{}, nil)[0].Handle

	grantResponse, err := handler(context.Background(), rolePresetInteraction("grant-preset", "role-id", "relay-user"))
	if err != nil {
		t.Fatalf("grant-preset returned error: %v", err)
	}
	if manager.method != "GrantRoleCapabilities" || len(manager.capabilities) != 2 {
		t.Fatalf("manager call = %+v, want relay preset grant", manager)
	}
	if !strings.Contains(grantResponse.Content, "Granted preset") || !strings.Contains(grantResponse.Content, "<@&role-id>") || !grantResponse.Ephemeral {
		t.Fatalf("grant response = %+v, want ephemeral success with role mention", grantResponse)
	}

	revokeResponse, err := handler(context.Background(), rolePresetInteraction("revoke-preset", "role-id", "relay-user"))
	if err != nil {
		t.Fatalf("revoke-preset returned error: %v", err)
	}
	if manager.method != "RevokeRoleCapabilities" || len(manager.capabilities) != 2 {
		t.Fatalf("manager call = %+v, want relay preset revoke", manager)
	}
	if !strings.Contains(revokeResponse.Content, "Revoked preset") || !strings.Contains(revokeResponse.Content, "<@&role-id>") || !revokeResponse.Ephemeral {
		t.Fatalf("revoke response = %+v, want ephemeral success with role mention", revokeResponse)
	}
}

func TestPermissionCommandDirectUserGrantUsesUserGroup(t *testing.T) {
	manager := &fakeGrantManager{}
	handler := PermissionCommands(manager, &fakeGuildRoleService{}, nil)[0].Handle

	response, err := handler(context.Background(), Interaction{
		GuildID: "guild-id",
		UserID:  "actor-id",
		Name:    "permissions",
		Options: []InteractionOption{{
			Name: "user",
			Options: []InteractionOption{{
				Name: "grant",
				Options: []InteractionOption{
					{Name: "user", Value: "user-id"},
					{Name: "capability", Value: "job.admin"},
				},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if manager.method != "GrantUser" || manager.targetID != "user-id" || manager.capability != "job.admin" {
		t.Fatalf("manager call = %+v, want direct user grant", manager)
	}
	if !strings.Contains(response.Content, "Granted `job.admin`") || !strings.Contains(response.Content, "<@user-id>") || !response.Ephemeral {
		t.Fatalf("response = %+v, want ephemeral success with user mention", response)
	}
}

func findOption(options []*discordgo.ApplicationCommandOption, name string) *discordgo.ApplicationCommandOption {
	for _, option := range options {
		if option.Name == name {
			return option
		}
	}
	return nil
}

func hasChoice(option *discordgo.ApplicationCommandOption, value string) bool {
	if option == nil {
		return false
	}
	for _, choice := range option.Choices {
		if choice.Value == value {
			return true
		}
	}
	return false
}

func roleInteraction(action string, roleID string, userID string) Interaction {
	return Interaction{
		GuildID: "guild-id",
		UserID:  "actor-id",
		Name:    "permissions",
		Options: []InteractionOption{{
			Name: "role",
			Options: []InteractionOption{{
				Name: action,
				Options: []InteractionOption{
					{Name: "role", Value: roleID},
					{Name: "user", Value: userID},
				},
			}},
		}},
	}
}

func rolePresetInteraction(action string, roleID string, preset string) Interaction {
	return Interaction{
		GuildID: "guild-id",
		UserID:  "actor-id",
		Name:    "permissions",
		Options: []InteractionOption{{
			Name: "role",
			Options: []InteractionOption{{
				Name: action,
				Options: []InteractionOption{
					{Name: "role", Value: roleID},
					{Name: "preset", Value: preset},
				},
			}},
		}},
	}
}

type fakeGrantManager struct {
	method       string
	guildID      string
	targetID     string
	capability   capability.Capability
	capabilities []capability.Capability
	actorID      string
	err          error
}

func (m *fakeGrantManager) GrantRole(_ context.Context, guildID string, roleID string, cap capability.Capability, actorID string) error {
	m.method, m.guildID, m.targetID, m.capability, m.actorID = "GrantRole", guildID, roleID, cap, actorID
	return m.err
}

func (m *fakeGrantManager) RevokeRole(_ context.Context, guildID string, roleID string, cap capability.Capability) error {
	m.method, m.guildID, m.targetID, m.capability = "RevokeRole", guildID, roleID, cap
	return m.err
}

func (m *fakeGrantManager) GrantRoleCapabilities(_ context.Context, guildID string, roleID string, caps []capability.Capability, actorID string) error {
	m.method, m.guildID, m.targetID, m.capabilities, m.actorID = "GrantRoleCapabilities", guildID, roleID, caps, actorID
	return m.err
}

func (m *fakeGrantManager) RevokeRoleCapabilities(_ context.Context, guildID string, roleID string, caps []capability.Capability) error {
	m.method, m.guildID, m.targetID, m.capabilities = "RevokeRoleCapabilities", guildID, roleID, caps
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

type fakeGuildRoleService struct {
	createdRoleID string
	createdName   string
	addedRoleID   string
	addedUserID   string
	removedRoleID string
	removedUserID string
	err           error
}

func (s *fakeGuildRoleService) GuildRoleCreate(_ string, data *discordgo.RoleParams, _ ...discordgo.RequestOption) (*discordgo.Role, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.createdName = data.Name
	return &discordgo.Role{ID: s.createdRoleID, Name: data.Name}, nil
}

func (s *fakeGuildRoleService) GuildMemberRoleAdd(_ string, userID string, roleID string, _ ...discordgo.RequestOption) error {
	if s.err != nil {
		return s.err
	}
	s.addedUserID = userID
	s.addedRoleID = roleID
	return nil
}

func (s *fakeGuildRoleService) GuildMemberRoleRemove(_ string, userID string, roleID string, _ ...discordgo.RequestOption) error {
	if s.err != nil {
		return s.err
	}
	s.removedUserID = userID
	s.removedRoleID = roleID
	return nil
}

type fakeAuditRecorder struct {
	events []audit.Event
	err    error
}

func (r *fakeAuditRecorder) Record(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return r.err
}
