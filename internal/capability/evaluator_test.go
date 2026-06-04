package capability

import (
	"context"
	"errors"
	"testing"
)

func TestEvaluatorAllowsGuildOwnerWithoutStoreLookup(t *testing.T) {
	store := &fakeGrantStore{err: errors.New("store should not be called")}
	evaluator := NewEvaluator(store)

	decision, err := evaluator.Check(context.Background(), Subject{
		GuildID:      "guild-id",
		UserID:       "owner-id",
		IsGuildOwner: true,
	}, "plugin.install")
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !decision.Allowed || decision.Reason != ReasonAdminOverride {
		t.Fatalf("decision = %+v, want admin override allow", decision)
	}
	if store.calls != 0 {
		t.Fatalf("store calls = %d, want 0", store.calls)
	}
}

func TestEvaluatorAllowsDiscordAdministratorWithoutStoreLookup(t *testing.T) {
	store := &fakeGrantStore{err: errors.New("store should not be called")}
	evaluator := NewEvaluator(store)

	decision, err := evaluator.Check(context.Background(), Subject{
		GuildID:          "guild-id",
		UserID:           "admin-id",
		HasAdministrator: true,
	}, "job.admin")
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !decision.Allowed || decision.Reason != ReasonAdminOverride {
		t.Fatalf("decision = %+v, want admin override allow", decision)
	}
	if store.calls != 0 {
		t.Fatalf("store calls = %d, want 0", store.calls)
	}
}

func TestEvaluatorAllowsDirectUserGrant(t *testing.T) {
	evaluator := NewEvaluator(&fakeGrantStore{
		userGrants: []Capability{"relay.dispatch"},
	})

	decision, err := evaluator.Check(context.Background(), Subject{
		GuildID: "guild-id",
		UserID:  "user-id",
	}, "relay.dispatch")
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !decision.Allowed || decision.Reason != ReasonUserGrant {
		t.Fatalf("decision = %+v, want user grant allow", decision)
	}
}

func TestEvaluatorAllowsRoleGrantByRoleID(t *testing.T) {
	evaluator := NewEvaluator(&fakeGrantStore{
		roleGrants: map[string][]Capability{
			"role-123": {"plugin.run.example"},
			"role-999": {"job.admin"},
		},
	})

	decision, err := evaluator.Check(context.Background(), Subject{
		GuildID: "guild-id",
		UserID:  "user-id",
		RoleIDs: []string{"role-123"},
	}, "plugin.run.example")
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !decision.Allowed || decision.Reason != ReasonRoleGrant {
		t.Fatalf("decision = %+v, want role grant allow", decision)
	}
}

func TestEvaluatorDeniesMissingCapability(t *testing.T) {
	evaluator := NewEvaluator(&fakeGrantStore{
		userGrants: []Capability{"relay.receive"},
		roleGrants: map[string][]Capability{
			"role-123": {"plugin.run.example"},
		},
	})

	decision, err := evaluator.Check(context.Background(), Subject{
		GuildID: "guild-id",
		UserID:  "user-id",
		RoleIDs: []string{"role-123"},
	}, "job.admin")
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if decision.Allowed || decision.Reason != ReasonMissingCapability {
		t.Fatalf("decision = %+v, want missing capability deny", decision)
	}
}

func TestEvaluatorFailsClosedWhenStoreFails(t *testing.T) {
	evaluator := NewEvaluator(&fakeGrantStore{err: errors.New("db down")})

	decision, err := evaluator.Check(context.Background(), Subject{
		GuildID: "guild-id",
		UserID:  "user-id",
		RoleIDs: []string{"role-123"},
	}, "plugin.install")
	if err == nil {
		t.Fatal("expected store error")
	}
	if decision.Allowed || decision.Reason != ReasonStoreError {
		t.Fatalf("decision = %+v, want store-error deny", decision)
	}
}

func TestEvaluatorFailsClosedWithoutGuildOrUser(t *testing.T) {
	evaluator := NewEvaluator(&fakeGrantStore{})

	for _, subject := range []Subject{
		{UserID: "user-id"},
		{GuildID: "guild-id"},
	} {
		decision, err := evaluator.Check(context.Background(), subject, "plugin.install")
		if err != nil {
			t.Fatalf("Check returned error: %v", err)
		}
		if decision.Allowed || decision.Reason != ReasonUnknownIdentity {
			t.Fatalf("decision = %+v, want unknown identity deny", decision)
		}
	}
}

type fakeGrantStore struct {
	userGrants []Capability
	roleGrants map[string][]Capability
	err        error
	calls      int
}

func (s *fakeGrantStore) UserCapabilities(ctx context.Context, guildID string, userID string) ([]Capability, error) {
	s.calls++
	return s.userGrants, s.err
}

func (s *fakeGrantStore) RoleCapabilities(ctx context.Context, guildID string, roleIDs []string) ([]Capability, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	var capabilities []Capability
	for _, roleID := range roleIDs {
		capabilities = append(capabilities, s.roleGrants[roleID]...)
	}
	return capabilities, nil
}
