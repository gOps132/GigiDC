package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/gOps132/GigiDC/internal/capability"
)

const ToolPermissionsCheck = "permissions.check"

type PermissionsCheckTool struct {
	Checker CapabilityChecker
}

func (t PermissionsCheckTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        ToolPermissionsCheck,
		Description: "Check whether the requesting user has a named Gigi capability in this guild.",
		Kind:        ToolKindRead,
	}
}

func (t PermissionsCheckTool) Execute(ctx context.Context, request Request, call ToolCall) (ToolResult, error) {
	if request.Surface != SurfaceGuildMention || request.GuildID == "" {
		return ToolResult{}, fmt.Errorf("guild permission context is required")
	}
	if t.Checker == nil {
		return ToolResult{}, fmt.Errorf("capability checker is required")
	}
	required, err := capability.Normalize(call.Args["capability"])
	if err != nil {
		return ToolResult{}, err
	}
	decision, err := t.Checker.Check(ctx, capability.Subject{
		GuildID:          request.GuildID,
		UserID:           request.ActorUserID,
		RoleIDs:          request.RoleIDs,
		HasAdministrator: request.HasAdministrator,
	}, required)
	if err != nil {
		return ToolResult{}, err
	}
	status := "does not have"
	if decision.Allowed {
		status = "has"
	}
	return ToolResult{
		Name:    ToolPermissionsCheck,
		Summary: fmt.Sprintf("You %s `%s` in this server.", status, safeInline(string(required))),
		Data: map[string]string{
			"capability": string(required),
			"allowed":    fmt.Sprintf("%t", decision.Allowed),
			"reason":     strings.TrimSpace(string(decision.Reason)),
		},
	}, nil
}
