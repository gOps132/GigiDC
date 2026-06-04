package capability

import (
	"context"
	"fmt"
	"strings"
	"unicode"
)

type Capability string

const CapabilityManage Capability = "capability.manage"

type Subject struct {
	GuildID          string
	UserID           string
	RoleIDs          []string
	IsGuildOwner     bool
	HasAdministrator bool
}

type Reason string

const (
	ReasonAdminOverride     Reason = "admin_override"
	ReasonUserGrant         Reason = "user_grant"
	ReasonRoleGrant         Reason = "role_grant"
	ReasonPublicAction      Reason = "public_action"
	ReasonMissingCapability Reason = "missing_capability"
	ReasonStoreError        Reason = "store_error"
	ReasonUnknownIdentity   Reason = "unknown_identity"
)

type Decision struct {
	Allowed    bool
	Capability Capability
	Reason     Reason
}

type GrantStore interface {
	UserCapabilities(ctx context.Context, guildID string, userID string) ([]Capability, error)
	RoleCapabilities(ctx context.Context, guildID string, roleIDs []string) ([]Capability, error)
}

type Evaluator struct {
	store GrantStore
}

func NewEvaluator(store GrantStore) Evaluator {
	return Evaluator{store: store}
}

func Normalize(value string) (Capability, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("capability is required")
	}
	if len(value) > 128 {
		return "", fmt.Errorf("capability is too long")
	}
	for _, r := range value {
		if unicode.IsLower(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '.', ':', '_', '-':
			continue
		default:
			return "", fmt.Errorf("capability contains unsupported character %q", r)
		}
	}
	return Capability(value), nil
}

func (e Evaluator) Check(ctx context.Context, subject Subject, capability Capability) (Decision, error) {
	capability = Capability(strings.TrimSpace(string(capability)))
	decision := Decision{Capability: capability, Reason: ReasonMissingCapability}

	if subject.IsGuildOwner || subject.HasAdministrator {
		decision.Allowed = true
		decision.Reason = ReasonAdminOverride
		return decision, nil
	}
	if strings.TrimSpace(subject.GuildID) == "" || strings.TrimSpace(subject.UserID) == "" || capability == "" {
		decision.Reason = ReasonUnknownIdentity
		return decision, nil
	}
	if e.store == nil {
		return decision, nil
	}

	userCaps, err := e.store.UserCapabilities(ctx, subject.GuildID, subject.UserID)
	if err != nil {
		decision.Reason = ReasonStoreError
		return decision, fmt.Errorf("load user capabilities: %w", err)
	}
	if contains(userCaps, capability) {
		decision.Allowed = true
		decision.Reason = ReasonUserGrant
		return decision, nil
	}

	roleCaps, err := e.store.RoleCapabilities(ctx, subject.GuildID, subject.RoleIDs)
	if err != nil {
		decision.Reason = ReasonStoreError
		return decision, fmt.Errorf("load role capabilities: %w", err)
	}
	if contains(roleCaps, capability) {
		decision.Allowed = true
		decision.Reason = ReasonRoleGrant
		return decision, nil
	}

	return decision, nil
}

func contains(capabilities []Capability, want Capability) bool {
	for _, capability := range capabilities {
		if Capability(strings.TrimSpace(string(capability))) == want {
			return true
		}
	}
	return false
}
