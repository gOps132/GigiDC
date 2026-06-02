package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gOps132/GigiDC/internal/capability"
)

type Source string

const (
	SourceEvent   Source = "event"
	SourceFetched Source = "fetched"
)

type Reason string

const (
	ReasonGuildRequired      Reason = "guild_required"
	ReasonMemberLookupFailed Reason = "member_lookup_failed"
	ReasonUserRequired       Reason = "user_required"
)

type Event struct {
	GuildID          string
	ChannelID        string
	UserID           string
	RoleIDs          []string
	IsGuildOwner     bool
	HasAdministrator bool
	RequireGuild     bool
}

type Member struct {
	UserID           string
	RoleIDs          []string
	IsGuildOwner     bool
	HasAdministrator bool
}

type Result struct {
	Resolved  bool
	Ambiguous bool
	Reason    Reason
	Source    Source
	Subject   capability.Subject
	ChannelID string
}

type MemberSource interface {
	Member(ctx context.Context, guildID string, userID string) (Member, error)
}

type Resolver struct {
	source MemberSource
}

func NewResolver(source MemberSource) Resolver {
	return Resolver{source: source}
}

func (r Resolver) Resolve(ctx context.Context, event Event) (Result, error) {
	event.GuildID = strings.TrimSpace(event.GuildID)
	event.ChannelID = strings.TrimSpace(event.ChannelID)
	event.UserID = strings.TrimSpace(event.UserID)

	result := Result{ChannelID: event.ChannelID}
	if event.UserID == "" {
		result.Ambiguous = true
		result.Reason = ReasonUserRequired
		return result, errors.New("identity user ID is required")
	}
	if event.RequireGuild && event.GuildID == "" {
		result.Ambiguous = true
		result.Reason = ReasonGuildRequired
		return result, errors.New("guild identity is required")
	}

	subject := capability.Subject{
		GuildID:          event.GuildID,
		UserID:           event.UserID,
		RoleIDs:          cleanRoleIDs(event.RoleIDs),
		IsGuildOwner:     event.IsGuildOwner,
		HasAdministrator: event.HasAdministrator,
	}
	if event.GuildID == "" || len(subject.RoleIDs) > 0 || subject.IsGuildOwner || subject.HasAdministrator || r.source == nil {
		result.Resolved = true
		result.Source = SourceEvent
		result.Subject = subject
		return result, nil
	}

	member, err := r.source.Member(ctx, event.GuildID, event.UserID)
	if err != nil {
		result.Ambiguous = true
		result.Reason = ReasonMemberLookupFailed
		return result, fmt.Errorf("lookup guild member: %w", err)
	}
	subject.RoleIDs = cleanRoleIDs(member.RoleIDs)
	subject.IsGuildOwner = member.IsGuildOwner
	subject.HasAdministrator = member.HasAdministrator
	result.Resolved = true
	result.Source = SourceFetched
	result.Subject = subject
	return result, nil
}

func cleanRoleIDs(roleIDs []string) []string {
	cleaned := make([]string, 0, len(roleIDs))
	seen := make(map[string]struct{}, len(roleIDs))
	for _, roleID := range roleIDs {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		if _, exists := seen[roleID]; exists {
			continue
		}
		seen[roleID] = struct{}{}
		cleaned = append(cleaned, roleID)
	}
	return cleaned
}
