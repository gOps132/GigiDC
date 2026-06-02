package identity

import (
	"context"
	"errors"
	"testing"
)

func TestResolverUsesEventMemberForGuildIdentity(t *testing.T) {
	source := &fakeMemberSource{err: errors.New("source should not be called")}
	resolver := NewResolver(source)

	result, err := resolver.Resolve(context.Background(), Event{
		GuildID:          "guild-id",
		ChannelID:        "channel-id",
		UserID:           "user-id",
		RoleIDs:          []string{"role-1", "role-2"},
		HasAdministrator: true,
	})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if !result.Resolved || result.Ambiguous || result.Subject.GuildID != "guild-id" || result.Subject.UserID != "user-id" {
		t.Fatalf("result = %+v, want resolved guild identity", result)
	}
	if !result.Subject.HasAdministrator || len(result.Subject.RoleIDs) != 2 {
		t.Fatalf("subject = %+v, want admin with roles", result.Subject)
	}
	if source.calls != 0 {
		t.Fatalf("source calls = %d, want 0", source.calls)
	}
}

func TestResolverFetchesMemberWhenRolesMissing(t *testing.T) {
	source := &fakeMemberSource{member: Member{
		UserID:           "user-id",
		RoleIDs:          []string{"role-1"},
		IsGuildOwner:     true,
		HasAdministrator: true,
	}}
	resolver := NewResolver(source)

	result, err := resolver.Resolve(context.Background(), Event{
		GuildID:   "guild-id",
		ChannelID: "channel-id",
		UserID:    "user-id",
	})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if !result.Resolved || result.Source != SourceFetched {
		t.Fatalf("result = %+v, want fetched identity", result)
	}
	if !result.Subject.IsGuildOwner || !result.Subject.HasAdministrator || len(result.Subject.RoleIDs) != 1 {
		t.Fatalf("subject = %+v, want fetched flags and roles", result.Subject)
	}
	if source.guildID != "guild-id" || source.userID != "user-id" {
		t.Fatalf("source lookup = %q/%q, want guild-id/user-id", source.guildID, source.userID)
	}
}

func TestResolverFailsClosedForGuildRequiredDM(t *testing.T) {
	resolver := NewResolver(&fakeMemberSource{})

	result, err := resolver.Resolve(context.Background(), Event{
		ChannelID:    "dm-channel",
		UserID:       "user-id",
		RequireGuild: true,
	})
	if err == nil {
		t.Fatal("expected guild-required error")
	}
	if result.Resolved || !result.Ambiguous || result.Reason != ReasonGuildRequired {
		t.Fatalf("result = %+v, want ambiguous guild-required result", result)
	}
}

func TestResolverFailsClosedWhenMemberFetchFails(t *testing.T) {
	resolver := NewResolver(&fakeMemberSource{err: errors.New("discord api down")})

	result, err := resolver.Resolve(context.Background(), Event{
		GuildID:   "guild-id",
		ChannelID: "channel-id",
		UserID:    "user-id",
	})
	if err == nil {
		t.Fatal("expected member source error")
	}
	if result.Resolved || !result.Ambiguous || result.Reason != ReasonMemberLookupFailed {
		t.Fatalf("result = %+v, want member lookup failure", result)
	}
}

func TestResolverFailsClosedWithoutUserID(t *testing.T) {
	resolver := NewResolver(&fakeMemberSource{})

	result, err := resolver.Resolve(context.Background(), Event{
		GuildID:   "guild-id",
		ChannelID: "channel-id",
	})
	if err == nil {
		t.Fatal("expected missing user error")
	}
	if result.Resolved || !result.Ambiguous || result.Reason != ReasonUserRequired {
		t.Fatalf("result = %+v, want missing user failure", result)
	}
}

type fakeMemberSource struct {
	member  Member
	err     error
	calls   int
	guildID string
	userID  string
}

func (s *fakeMemberSource) Member(ctx context.Context, guildID string, userID string) (Member, error) {
	s.calls++
	s.guildID = guildID
	s.userID = userID
	return s.member, s.err
}
