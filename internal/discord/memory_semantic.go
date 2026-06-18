package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/gOps132/GigiDC/internal/assistant"
	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/llm/provider"
	"github.com/gOps132/GigiDC/internal/memory"
)

type SemanticMemoryPlanner interface {
	Plan(ctx context.Context, input assistant.SemanticMemoryInput) (assistant.MemoryPlan, bool, error)
}

func SemanticMemoryHandler(manager MemoryManager, checker CapabilityChecker, recorder AuditRecorder, policy LLMPolicyManager, planner SemanticMemoryPlanner, fallback MessageHandler) MessageHandler {
	if fallback == nil {
		fallback = CoreMessageHandler()
	}
	return semanticMemoryHandler{
		manager:  manager,
		checker:  checker,
		recorder: recorder,
		policy:   policy,
		planner:  planner,
		fallback: fallback,
	}
}

type semanticMemoryHandler struct {
	manager  MemoryManager
	checker  CapabilityChecker
	recorder AuditRecorder
	policy   LLMPolicyManager
	planner  SemanticMemoryPlanner
	fallback MessageHandler
}

func (h semanticMemoryHandler) HandleMessage(ctx context.Context, message Message) (MessageResponse, error) {
	if message.Surface != MessageSurfaceGuildMention || strings.TrimSpace(message.GuildID) == "" || h.planner == nil {
		return h.fallback.HandleMessage(ctx, message)
	}
	mode, err := h.toolRoutingMode(ctx, message.GuildID)
	if err != nil {
		_ = h.record(ctx, message, assistant.MemoryPlan{}, audit.StatusFailed, "routing_policy_failed")
		return MessageResponse{Content: "Memory routing failed."}, nil
	}
	if mode == provider.ToolRoutingOff {
		return h.fallback.HandleMessage(ctx, message)
	}
	plan, ok, err := h.planner.Plan(ctx, assistant.SemanticMemoryInput{
		GuildID:     message.GuildID,
		ChannelID:   message.ChannelID,
		ActorUserID: message.UserID,
		Text:        message.Text,
	})
	if err != nil {
		_ = h.record(ctx, message, plan, audit.StatusFailed, "semantic_memory_failed")
		return MessageResponse{Content: "Memory routing failed."}, nil
	}
	if !ok {
		return h.fallback.HandleMessage(ctx, message)
	}
	if mode == provider.ToolRoutingDryRun {
		_ = h.record(ctx, message, plan, audit.StatusSucceeded, "dry_run")
		return MessageResponse{Content: formatSemanticMemoryDryRun(plan)}, nil
	}
	if h.manager == nil {
		_ = h.record(ctx, message, plan, audit.StatusFailed, "memory_manager_missing")
		return MessageResponse{Content: "Memory is not configured yet."}, nil
	}
	if h.checker == nil {
		_ = h.record(ctx, message, plan, audit.StatusFailed, "memory_checker_missing")
		return MessageResponse{Content: "Permission check failed."}, nil
	}
	decision, err := h.checker.Check(ctx, capability.Subject{
		GuildID:          message.GuildID,
		UserID:           message.UserID,
		RoleIDs:          message.RoleIDs,
		HasAdministrator: message.HasAdministrator,
	}, capability.Capability("memory.read.guild"))
	if err != nil {
		_ = h.record(ctx, message, plan, audit.StatusFailed, string(decision.Reason))
		return MessageResponse{Content: "Permission check failed."}, nil
	}
	if !decision.Allowed {
		_ = h.record(ctx, message, plan, audit.StatusDenied, string(decision.Reason))
		return MessageResponse{Content: "Permission denied for memory."}, nil
	}
	response, err := h.executePlan(ctx, message, plan)
	if err != nil {
		_ = h.record(ctx, message, plan, audit.StatusFailed, "memory_execute_failed")
		return MessageResponse{Content: "Memory lookup failed."}, nil
	}
	if err := h.record(ctx, message, plan, audit.StatusSucceeded, ""); err != nil {
		return MessageResponse{}, err
	}
	return response, nil
}

func (h semanticMemoryHandler) executePlan(ctx context.Context, message Message, plan assistant.MemoryPlan) (MessageResponse, error) {
	switch plan.Intent {
	case assistant.MemoryIntentCount:
		result, err := h.manager.CountMentions(ctx, memory.CountRequest{
			GuildID:      message.GuildID,
			ChannelID:    message.ChannelID,
			AuthorUserID: plan.TargetUserID,
			Text:         plan.Text,
		})
		if err != nil {
			return MessageResponse{}, err
		}
		return MessageResponse{Content: fmt.Sprintf("<@%s> mentioned `%s` %d times in this channel.", safeInline(plan.TargetUserID), safeInline(memory.NormalizeText(plan.Text)), result.Count)}, nil
	case assistant.MemoryIntentSearch:
		results, err := h.manager.SearchMessages(ctx, memory.SearchRequest{
			GuildID:   message.GuildID,
			ChannelID: message.ChannelID,
			Query:     plan.Query,
			Limit:     plan.Limit,
		})
		if err != nil {
			return MessageResponse{}, err
		}
		return MessageResponse{Content: formatMemorySearch(results, memoryRequest{Query: plan.Query})}, nil
	default:
		return MessageResponse{}, fmt.Errorf("unsupported memory intent")
	}
}

func (h semanticMemoryHandler) toolRoutingMode(ctx context.Context, guildID string) (provider.ToolRoutingMode, error) {
	if h.policy == nil {
		return provider.ToolRoutingOff, nil
	}
	policy, err := h.policy.GuildPolicy(ctx, guildID)
	if err != nil {
		return "", err
	}
	return policy.ToolRoutingMode, nil
}

func (h semanticMemoryHandler) record(ctx context.Context, message Message, plan assistant.MemoryPlan, status audit.Status, reason string) error {
	if h.recorder == nil || strings.TrimSpace(message.UserID) == "" {
		return nil
	}
	metadata := map[string]string{"source": "semantic_memory"}
	if plan.Intent != "" {
		metadata["intent"] = string(plan.Intent)
	}
	if plan.TargetUserID != "" {
		metadata["target_user_id"] = plan.TargetUserID
	}
	if plan.Scope != "" {
		metadata["scope"] = plan.Scope
	}
	return h.recorder.Record(ctx, audit.Event{
		Kind:     "discord.memory.semantic",
		GuildID:  message.GuildID,
		ActorID:  message.UserID,
		Status:   status,
		Reason:   reason,
		Metadata: metadata,
	})
}

func formatSemanticMemoryDryRun(plan assistant.MemoryPlan) string {
	switch plan.Intent {
	case assistant.MemoryIntentCount:
		return fmt.Sprintf("Planned memory count for <@%s> mentioning `%s`. LLM tool routing is in `dry-run` mode.", safeInline(plan.TargetUserID), safeInline(memory.NormalizeText(plan.Text)))
	case assistant.MemoryIntentSearch:
		return fmt.Sprintf("Planned memory search for `%s`. LLM tool routing is in `dry-run` mode.", safeInline(memory.NormalizeText(plan.Query)))
	default:
		return "Planned memory tool call. LLM tool routing is in `dry-run` mode."
	}
}
