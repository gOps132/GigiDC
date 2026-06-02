package discord

import (
	"context"
	"fmt"

	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/identity"
)

type CapabilityChecker interface {
	Check(ctx context.Context, subject capability.Subject, required capability.Capability) (capability.Decision, error)
}

type AuditRecorder interface {
	Record(ctx context.Context, event audit.Event) error
}

type CapabilityAuthorizer struct {
	resolver identity.Resolver
	checker  CapabilityChecker
	audit    AuditRecorder
}

func NewCapabilityAuthorizer(checker CapabilityChecker, recorder AuditRecorder) CapabilityAuthorizer {
	return CapabilityAuthorizer{
		resolver: identity.NewResolver(nil),
		checker:  checker,
		audit:    recorder,
	}
}

func (a CapabilityAuthorizer) Check(ctx context.Context, interaction Interaction, required capability.Capability) (capability.Decision, error) {
	result, resolveErr := a.resolver.Resolve(ctx, identity.Event{
		GuildID:          interaction.GuildID,
		ChannelID:        interaction.ChannelID,
		UserID:           interaction.UserID,
		RoleIDs:          interaction.RoleIDs,
		HasAdministrator: interaction.HasAdministrator,
		RequireGuild:     true,
	})
	if resolveErr != nil {
		decision := capability.Decision{Allowed: false, Capability: required, Reason: capability.ReasonUnknownIdentity}
		return decision, a.record(ctx, interaction, required, decision, resolveErr)
	}
	if a.checker == nil {
		decision := capability.Decision{Allowed: false, Capability: required, Reason: capability.ReasonStoreError}
		return decision, a.record(ctx, interaction, required, decision, fmt.Errorf("capability checker is required"))
	}

	decision, err := a.checker.Check(ctx, result.Subject, required)
	if err != nil {
		return decision, a.record(ctx, interaction, required, decision, err)
	}
	return decision, a.record(ctx, interaction, required, decision, nil)
}

func (a CapabilityAuthorizer) record(ctx context.Context, interaction Interaction, required capability.Capability, decision capability.Decision, checkErr error) error {
	if a.audit == nil {
		return checkErr
	}
	status := audit.StatusDenied
	if decision.Allowed {
		status = audit.StatusAllowed
	}
	if checkErr != nil {
		status = audit.StatusFailed
	}
	recordErr := a.audit.Record(ctx, audit.Event{
		Kind:    "discord.permission.check",
		GuildID: interaction.GuildID,
		ActorID: interaction.UserID,
		Status:  status,
		Reason:  string(decision.Reason),
		Metadata: map[string]string{
			"capability": string(required),
			"command":    interaction.Name,
		},
	})
	if recordErr != nil {
		if checkErr != nil {
			return fmt.Errorf("%w; record permission audit: %v", checkErr, recordErr)
		}
		return fmt.Errorf("record permission audit: %w", recordErr)
	}
	return checkErr
}
