package agent

import (
	"context"
	"fmt"

	"github.com/gOps132/GigiDC/internal/capability"
	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
)

type RoutingPolicy struct {
	Policy                       PolicyManager
	Checker                      CapabilityChecker
	RequiredCapabilityBeforePlan capability.Capability
}

func (p RoutingPolicy) Mode(ctx context.Context, guildID string) (llmprovider.ToolRoutingMode, error) {
	if p.Policy == nil {
		return llmprovider.ToolRoutingOff, nil
	}
	policy, err := p.Policy.GuildPolicy(ctx, guildID)
	if err != nil {
		return "", err
	}
	return policy.ToolRoutingMode, nil
}

func (p RoutingPolicy) CheckBeforePlan(ctx context.Context, request Request) (capability.Decision, error) {
	if p.RequiredCapabilityBeforePlan == "" {
		return capability.Decision{Allowed: true, Reason: capability.ReasonPublicAction}, nil
	}
	return p.checkCapability(ctx, request, p.RequiredCapabilityBeforePlan)
}

func (p RoutingPolicy) checkCapability(ctx context.Context, request Request, required capability.Capability) (capability.Decision, error) {
	if p.Checker == nil {
		return capability.Decision{Allowed: false, Capability: required, Reason: capability.ReasonStoreError}, fmt.Errorf("capability checker is required")
	}
	return p.Checker.Check(ctx, capability.Subject{
		GuildID:          request.GuildID,
		UserID:           request.ActorUserID,
		RoleIDs:          request.RoleIDs,
		HasAdministrator: request.HasAdministrator,
	}, required)
}
