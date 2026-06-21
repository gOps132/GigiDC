package discord

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/gOps132/GigiDC/internal/agent"
	"github.com/gOps132/GigiDC/internal/audit"
)

func TestAgentCommandsExposeRunManagementSurface(t *testing.T) {
	commands := AgentCommands(nil, &fakeAgentRunManager{}, nil)
	if len(commands) != 1 || commands[0].Name != "agent" {
		t.Fatalf("commands=%+v, want agent command", commands)
	}
	for _, name := range []string{"trace", "cancel", "pending", "confirm", "reject"} {
		if findOption(commands[0].Options, name) == nil {
			t.Fatalf("options=%+v, want %s", commands[0].Options, name)
		}
	}
}

func TestAgentCommandCancelsOwnedRunningRun(t *testing.T) {
	manager := &fakeAgentRunManager{run: agent.RunRecord{ID: "run-1", GuildID: "guild-id", ActorUserID: "actor-id", Status: agent.RunStatusRunning}}
	recorder := &fakeAuditRecorder{}
	handler := AgentCommands(nil, manager, recorder)[0].Handle

	response, err := handler(context.Background(), agentInteraction("cancel", "run-1", "actor-id", false))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !strings.Contains(response.Content, "Cancellation requested") || !response.Ephemeral {
		t.Fatalf("response=%+v, want cancellation response", response)
	}
	if manager.canceledRunID != "run-1" || manager.canceledActorID != "actor-id" {
		t.Fatalf("manager=%+v, want cancel request", manager)
	}
	if len(recorder.events) != 1 || recorder.events[0].Kind != "discord.agent.cancel" || recorder.events[0].Status != audit.StatusSucceeded {
		t.Fatalf("events=%+v, want succeeded cancel audit", recorder.events)
	}
}

func TestAgentCommandShowsAndConfirmsPendingAction(t *testing.T) {
	manager := &fakeAgentRunManager{
		run: agent.RunRecord{ID: "run-1", GuildID: "guild-id", ActorUserID: "actor-id", Status: agent.RunStatusConfirmationRequired},
		confirmation: agent.ConfirmationRecord{
			ID:              "confirm-1",
			RunID:           "run-1",
			StepIndex:       2,
			Status:          agent.ConfirmationStatusPending,
			ToolName:        "plugin.dispatch",
			Payload:         map[string]string{"arg.target": "music", "api_key": "redacted"},
			CreatedByUserID: "actor-id",
		},
		hasConfirmation: true,
	}
	handler := AgentCommands(nil, manager, nil)[0].Handle

	pending, err := handler(context.Background(), agentInteraction("pending", "run-1", "actor-id", false))
	if err != nil {
		t.Fatalf("pending returned error: %v", err)
	}
	if !strings.Contains(pending.Content, "plugin.dispatch") || !strings.Contains(pending.Content, "/agent confirm run:run-1") {
		t.Fatalf("pending response=%q, want confirmation details", pending.Content)
	}

	confirmed, err := handler(context.Background(), agentInteraction("confirm", "run-1", "actor-id", false))
	if err != nil {
		t.Fatalf("confirm returned error: %v", err)
	}
	if !strings.Contains(confirmed.Content, "Confirmed pending agent action") {
		t.Fatalf("confirm response=%q, want confirmed message", confirmed.Content)
	}
	if manager.resolvedConfirmationID != "confirm-1" || manager.resolvedStatus != agent.ConfirmationStatusConfirmed || manager.resolvedActorID != "actor-id" {
		t.Fatalf("manager=%+v, want confirmed pending action", manager)
	}
}

func TestAgentCommandRejectsOtherUserRun(t *testing.T) {
	manager := &fakeAgentRunManager{run: agent.RunRecord{ID: "run-1", GuildID: "guild-id", ActorUserID: "owner-id", Status: agent.RunStatusRunning}}
	recorder := &fakeAuditRecorder{}
	handler := AgentCommands(nil, manager, recorder)[0].Handle

	response, err := handler(context.Background(), agentInteraction("cancel", "run-1", "actor-id", false))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !strings.Contains(response.Content, "Permission denied") || !response.Ephemeral {
		t.Fatalf("response=%+v, want permission denial", response)
	}
	if manager.canceledRunID != "" {
		t.Fatalf("manager=%+v, want no cancel", manager)
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusDenied {
		t.Fatalf("events=%+v, want denied audit", recorder.events)
	}
}

func TestAgentCommandAllowsAdministratorForOtherUserRun(t *testing.T) {
	manager := &fakeAgentRunManager{run: agent.RunRecord{ID: "run-1", GuildID: "guild-id", ActorUserID: "owner-id", Status: agent.RunStatusRunning}}
	handler := AgentCommands(nil, manager, nil)[0].Handle

	response, err := handler(context.Background(), agentInteraction("cancel", "run-1", "admin-id", true))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !strings.Contains(response.Content, "Cancellation requested") {
		t.Fatalf("response=%+v, want admin cancellation", response)
	}
	if manager.canceledRunID != "run-1" || manager.canceledActorID != "admin-id" {
		t.Fatalf("manager=%+v, want admin cancel", manager)
	}
}

func agentInteraction(action string, runID string, userID string, admin bool) Interaction {
	return Interaction{
		GuildID:          "guild-id",
		UserID:           userID,
		HasAdministrator: admin,
		Name:             "agent",
		Options: []InteractionOption{{
			Name: action,
			Options: []InteractionOption{
				{Name: "run", Value: runID},
			},
		}},
	}
}

type fakeAgentRunManager struct {
	run                    agent.RunRecord
	loadOK                 bool
	err                    error
	confirmation           agent.ConfirmationRecord
	hasConfirmation        bool
	canceledRunID          string
	canceledActorID        string
	resolvedConfirmationID string
	resolvedStatus         agent.ConfirmationStatus
	resolvedActorID        string
}

func (m *fakeAgentRunManager) LoadRun(context.Context, string) (agent.RunRecord, bool, error) {
	if m.err != nil {
		return agent.RunRecord{}, false, m.err
	}
	if m.run.ID == "" && !m.loadOK {
		return agent.RunRecord{}, false, nil
	}
	return m.run, true, nil
}

func (m *fakeAgentRunManager) RequestCancelRun(_ context.Context, runID string, actorID string) error {
	if m.err != nil {
		return m.err
	}
	if m.run.Status != agent.RunStatusRunning {
		return sql.ErrNoRows
	}
	m.canceledRunID = runID
	m.canceledActorID = actorID
	return nil
}

func (m *fakeAgentRunManager) PendingConfirmation(context.Context, string) (agent.ConfirmationRecord, bool, error) {
	if m.err != nil {
		return agent.ConfirmationRecord{}, false, m.err
	}
	return m.confirmation, m.hasConfirmation, nil
}

func (m *fakeAgentRunManager) ResolveConfirmation(_ context.Context, confirmationID string, status agent.ConfirmationStatus, actorID string) error {
	if m.err != nil {
		return m.err
	}
	m.resolvedConfirmationID = confirmationID
	m.resolvedStatus = status
	m.resolvedActorID = actorID
	return nil
}
