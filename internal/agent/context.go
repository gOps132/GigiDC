package agent

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/contextbroker"
)

var ErrContextPermissionDenied = errors.New("permission denied for context")

type ContextFetcher interface {
	FetchContext(context.Context, Request) (contextbroker.Pack, error)
}

type ChannelContextSource interface {
	Fetch(context.Context, contextbroker.ChannelRecentRequest) (contextbroker.Pack, error)
}

type ChannelContextFetcher struct {
	Source   ChannelContextSource
	Checker  CapabilityChecker
	Limit    int
	MaxChars int
}

func (f ChannelContextFetcher) FetchContext(ctx context.Context, request Request) (contextbroker.Pack, error) {
	if !isChannelContextScope(request.ContextScope) {
		return contextbroker.Pack{}, nil
	}
	if request.Surface != SurfaceGuildMention || request.GuildID == "" || request.ChannelID == "" {
		return contextbroker.Pack{}, fmt.Errorf("guild channel context is required")
	}
	if f.Source == nil {
		return contextbroker.Pack{}, fmt.Errorf("context source is required")
	}
	if f.Checker == nil {
		return contextbroker.Pack{}, fmt.Errorf("capability checker is required")
	}
	decision, err := f.Checker.Check(ctx, capability.Subject{
		GuildID:          request.GuildID,
		UserID:           request.ActorUserID,
		RoleIDs:          request.RoleIDs,
		HasAdministrator: request.HasAdministrator,
	}, capability.Capability("memory.read.guild"))
	if err != nil {
		return contextbroker.Pack{}, err
	}
	if !decision.Allowed {
		return contextbroker.Pack{}, ErrContextPermissionDenied
	}
	return f.Source.Fetch(ctx, contextbroker.ChannelRecentRequest{
		GuildID:   request.GuildID,
		ChannelID: request.ChannelID,
		Limit:     f.limit(),
		MaxChars:  f.maxChars(),
	})
}

func (f ChannelContextFetcher) limit() int {
	if f.Limit > 0 {
		return f.Limit
	}
	return 10
}

func (f ChannelContextFetcher) maxChars() int {
	if f.MaxChars > 0 {
		return f.MaxChars
	}
	return 2500
}

func contextMetadata(pack contextbroker.Pack) map[string]string {
	return map[string]string{
		"scope":      "channel",
		"source":     contextbroker.SourceMemoryCurrentChannel,
		"snippets":   strconv.Itoa(len(pack.Snippets)),
		"chars":      strconv.Itoa(pack.Chars),
		"truncated":  strconv.FormatBool(pack.Truncated),
		"capability": "memory.read.guild",
	}
}
