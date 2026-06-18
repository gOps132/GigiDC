package discord

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gOps132/GigiDC/internal/assistant"
	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/llm/provider"
	"github.com/gOps132/GigiDC/internal/memory"
)

func TestSemanticMemoryHandlerFallsBackWhenRoutingOff(t *testing.T) {
	handler := SemanticMemoryHandler(&fakeMemoryManager{}, &fakeCapabilityChecker{}, nil, &fakeLLMPolicyManager{policy: provider.GuildPolicy{GuildID: "guild-id", ToolRoutingMode: provider.ToolRoutingOff}}, &fakeSemanticMemoryPlanner{}, MessageHandlerFunc(func(context.Context, Message) (MessageResponse, error) {
		return MessageResponse{Content: "fallback"}, nil
	}))

	response, err := handler.HandleMessage(context.Background(), semanticMemoryMessage("anything"))
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content != "fallback" {
		t.Fatalf("response = %q, want fallback", response.Content)
	}
}

func TestSemanticMemoryHandlerDryRunDoesNotExecute(t *testing.T) {
	manager := &fakeMemoryManager{count: memory.CountResult{Count: 9}}
	recorder := &fakeAuditRecorder{}
	handler := SemanticMemoryHandler(manager, &fakeCapabilityChecker{}, recorder, &fakeLLMPolicyManager{policy: provider.GuildPolicy{GuildID: "guild-id", ToolRoutingMode: provider.ToolRoutingDryRun}}, &fakeSemanticMemoryPlanner{
		plan: assistant.MemoryPlan{Intent: assistant.MemoryIntentCount, TargetUserID: "123", Text: "postgres", Scope: "this-channel"},
		ok:   true,
	}, CoreMessageHandler())

	response, err := handler.HandleMessage(context.Background(), semanticMemoryMessage(`wassup how many times did <@123> mentioned "postgres"?`))
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if manager.method != "" {
		t.Fatalf("manager method = %q, want no execution in dry-run", manager.method)
	}
	if !strings.Contains(response.Content, "Planned memory count") || !strings.Contains(response.Content, "dry-run") {
		t.Fatalf("response = %q, want dry-run plan", response.Content)
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusSucceeded || recorder.events[0].Reason != "dry_run" {
		t.Fatalf("audit events = %+v, want dry-run audit", recorder.events)
	}
}

func TestSemanticMemoryHandlerEnabledExecutesCount(t *testing.T) {
	manager := &fakeMemoryManager{count: memory.CountResult{Count: 3}}
	recorder := &fakeAuditRecorder{}
	handler := SemanticMemoryHandler(manager, &fakeCapabilityChecker{decision: capability.Decision{Allowed: true, Capability: "memory.read.guild", Reason: capability.ReasonRoleGrant}}, recorder, &fakeLLMPolicyManager{policy: provider.GuildPolicy{GuildID: "guild-id", ToolRoutingMode: provider.ToolRoutingEnabled}}, &fakeSemanticMemoryPlanner{
		plan: assistant.MemoryPlan{Intent: assistant.MemoryIntentCount, TargetUserID: "123", Text: "postgres", Scope: "this-channel"},
		ok:   true,
	}, CoreMessageHandler())

	response, err := handler.HandleMessage(context.Background(), semanticMemoryMessage(`wassup how many times did <@123> mentioned "postgres"?`))
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if manager.method != "CountMentions" || manager.countReq.AuthorUserID != "123" || manager.countReq.Text != "postgres" {
		t.Fatalf("manager = %+v, want count execution", manager)
	}
	if response.Content != "<@123> mentioned `postgres` 3 times in this channel." {
		t.Fatalf("response = %q, want count answer", response.Content)
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusSucceeded || recorder.events[0].Metadata["intent"] != "count" {
		t.Fatalf("audit events = %+v, want semantic memory audit", recorder.events)
	}
	if _, ok := recorder.events[0].Metadata["text"]; ok {
		t.Fatalf("audit metadata = %+v, must not store raw query", recorder.events[0].Metadata)
	}
}

func TestSemanticMemoryHandlerEnabledExecutesChannelCount(t *testing.T) {
	manager := &fakeMemoryManager{count: memory.CountResult{Count: 7}}
	handler := SemanticMemoryHandler(manager, &fakeCapabilityChecker{decision: capability.Decision{Allowed: true, Capability: "memory.read.guild", Reason: capability.ReasonRoleGrant}}, nil, &fakeLLMPolicyManager{policy: provider.GuildPolicy{GuildID: "guild-id", ToolRoutingMode: provider.ToolRoutingEnabled}}, &fakeSemanticMemoryPlanner{
		plan: assistant.MemoryPlan{Intent: assistant.MemoryIntentCount, Text: "postgres", Scope: "this-channel"},
		ok:   true,
	}, CoreMessageHandler())

	response, err := handler.HandleMessage(context.Background(), semanticMemoryMessage("how many times has postgres been mentioned in this channel?"))
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if manager.method != "CountMentions" || manager.countReq.AuthorUserID != "" || manager.countReq.Text != "postgres" {
		t.Fatalf("manager = %+v, want channel count execution", manager)
	}
	if response.Content != "Messages mentioned `postgres` 7 times in this channel." {
		t.Fatalf("response = %q, want channel count answer", response.Content)
	}
}

func TestSemanticMemoryHandlerEnabledExecutesSearch(t *testing.T) {
	manager := &fakeMemoryManager{searchResults: []memory.SearchResult{{AuthorUserID: "123", Text: "postgres outage notes"}}}
	handler := SemanticMemoryHandler(manager, &fakeCapabilityChecker{decision: capability.Decision{Allowed: true, Capability: "memory.read.guild", Reason: capability.ReasonRoleGrant}}, nil, &fakeLLMPolicyManager{policy: provider.GuildPolicy{GuildID: "guild-id", ToolRoutingMode: provider.ToolRoutingEnabled}}, &fakeSemanticMemoryPlanner{
		plan: assistant.MemoryPlan{Intent: assistant.MemoryIntentSearch, Query: "postgres outage", Scope: "this-channel", Limit: 5},
		ok:   true,
	}, CoreMessageHandler())

	response, err := handler.HandleMessage(context.Background(), semanticMemoryMessage("what did we say about postgres outage"))
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if manager.method != "SearchMessages" || manager.searchReq.Query != "postgres outage" {
		t.Fatalf("manager = %+v, want search execution", manager)
	}
	if !strings.Contains(response.Content, "Memory search for `postgres outage`:") {
		t.Fatalf("response = %q, want search response", response.Content)
	}
}

func TestSemanticMemoryHandlerDeniedCapability(t *testing.T) {
	handler := SemanticMemoryHandler(&fakeMemoryManager{}, &fakeCapabilityChecker{decision: capability.Decision{Allowed: false, Capability: "memory.read.guild", Reason: capability.ReasonMissingCapability}}, nil, &fakeLLMPolicyManager{policy: provider.GuildPolicy{GuildID: "guild-id", ToolRoutingMode: provider.ToolRoutingEnabled}}, &fakeSemanticMemoryPlanner{
		plan: assistant.MemoryPlan{Intent: assistant.MemoryIntentCount, TargetUserID: "123", Text: "postgres", Scope: "this-channel"},
		ok:   true,
	}, CoreMessageHandler())

	response, err := handler.HandleMessage(context.Background(), semanticMemoryMessage("count postgres"))
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content != "Permission denied for memory." {
		t.Fatalf("response = %q, want permission denial", response.Content)
	}
}

func TestSemanticMemoryHandlerFallsBackWhenNoPlan(t *testing.T) {
	handler := SemanticMemoryHandler(&fakeMemoryManager{}, &fakeCapabilityChecker{}, nil, &fakeLLMPolicyManager{policy: provider.GuildPolicy{GuildID: "guild-id", ToolRoutingMode: provider.ToolRoutingEnabled}}, &fakeSemanticMemoryPlanner{}, MessageHandlerFunc(func(context.Context, Message) (MessageResponse, error) {
		return MessageResponse{Content: "fallback"}, nil
	}))

	response, err := handler.HandleMessage(context.Background(), semanticMemoryMessage("hello"))
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content != "fallback" {
		t.Fatalf("response = %q, want fallback", response.Content)
	}
}

func TestSemanticMemoryHandlerMasksPlannerError(t *testing.T) {
	handler := SemanticMemoryHandler(&fakeMemoryManager{}, &fakeCapabilityChecker{}, nil, &fakeLLMPolicyManager{policy: provider.GuildPolicy{GuildID: "guild-id", ToolRoutingMode: provider.ToolRoutingEnabled}}, &fakeSemanticMemoryPlanner{err: errors.New("routing down")}, CoreMessageHandler())

	response, err := handler.HandleMessage(context.Background(), semanticMemoryMessage("count postgres"))
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content != "Memory routing failed." {
		t.Fatalf("response = %q, want safe failure", response.Content)
	}
}

func semanticMemoryMessage(text string) Message {
	return Message{
		Surface:   MessageSurfaceGuildMention,
		GuildID:   "guild-id",
		ChannelID: "channel-id",
		UserID:    "actor-id",
		Text:      text,
	}
}

type fakeSemanticMemoryPlanner struct {
	input assistant.SemanticMemoryInput
	plan  assistant.MemoryPlan
	ok    bool
	err   error
}

func (p *fakeSemanticMemoryPlanner) Plan(ctx context.Context, input assistant.SemanticMemoryInput) (assistant.MemoryPlan, bool, error) {
	p.input = input
	return p.plan, p.ok, p.err
}
