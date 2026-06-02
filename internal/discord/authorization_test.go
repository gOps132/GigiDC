package discord

import (
	"context"
	"errors"
	"testing"

	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
)

func TestCapabilityAuthorizerAllowsAndAudits(t *testing.T) {
	checker := &fakeCapabilityChecker{
		decision: capability.Decision{Allowed: true, Capability: "job.admin", Reason: capability.ReasonRoleGrant},
	}
	recorder := &fakeAuthAuditRecorder{}
	authorizer := NewCapabilityAuthorizer(checker, recorder)

	decision, err := authorizer.Check(context.Background(), Interaction{
		GuildID: "guild-id",
		UserID:  "user-id",
		RoleIDs: []string{"role-id"},
		Name:    "permissions",
	}, "job.admin")
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("decision = %+v, want allowed", decision)
	}
	if checker.subject.GuildID != "guild-id" || checker.subject.UserID != "user-id" || len(checker.subject.RoleIDs) != 1 {
		t.Fatalf("subject = %+v, want resolved interaction", checker.subject)
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusAllowed {
		t.Fatalf("audit events = %+v, want allowed", recorder.events)
	}
}

func TestCapabilityAuthorizerFailsClosedOnMissingGuild(t *testing.T) {
	recorder := &fakeAuthAuditRecorder{}
	authorizer := NewCapabilityAuthorizer(&fakeCapabilityChecker{}, recorder)

	decision, err := authorizer.Check(context.Background(), Interaction{
		UserID: "user-id",
		Name:   "permissions",
	}, "capability.manage")
	if err == nil {
		t.Fatal("expected resolver error")
	}
	if decision.Allowed {
		t.Fatalf("decision = %+v, want denied", decision)
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusFailed {
		t.Fatalf("audit events = %+v, want failed", recorder.events)
	}
}

func TestCapabilityAuthorizerReturnsCheckerErrors(t *testing.T) {
	authorizer := NewCapabilityAuthorizer(&fakeCapabilityChecker{
		decision: capability.Decision{Allowed: false, Capability: "job.admin", Reason: capability.ReasonStoreError},
		err:      errors.New("store failed"),
	}, nil)

	_, err := authorizer.Check(context.Background(), Interaction{
		GuildID: "guild-id",
		UserID:  "user-id",
		Name:    "permissions",
	}, "job.admin")
	if err == nil {
		t.Fatal("expected checker error")
	}
}

type fakeCapabilityChecker struct {
	subject  capability.Subject
	decision capability.Decision
	err      error
}

func (c *fakeCapabilityChecker) Check(_ context.Context, subject capability.Subject, required capability.Capability) (capability.Decision, error) {
	c.subject = subject
	if c.decision.Capability == "" {
		c.decision.Capability = required
	}
	return c.decision, c.err
}

type fakeAuthAuditRecorder struct {
	events []audit.Event
}

func (r *fakeAuthAuditRecorder) Record(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}
