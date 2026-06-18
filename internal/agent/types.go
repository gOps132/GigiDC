package agent

import (
	"context"
	"strings"
)

type Surface string

const (
	SurfaceDM           Surface = "dm"
	SurfaceGuildMention Surface = "guild_mention"
)

type Visibility string

const (
	VisibilityDefault Visibility = ""
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

type Request struct {
	Surface          Surface
	GuildID          string
	ChannelID        string
	ActorUserID      string
	RoleIDs          []string
	HasAdministrator bool
	Text             string
	RawText          string
}

type Response struct {
	Text       string
	Visibility Visibility
}

type Handler interface {
	HandleAgentRequest(context.Context, Request) (Response, bool, error)
}

type HandlerFunc func(context.Context, Request) (Response, bool, error)

func (f HandlerFunc) HandleAgentRequest(ctx context.Context, request Request) (Response, bool, error) {
	return f(ctx, request)
}

type Runtime struct {
	Handlers []Handler
	Fallback Handler
}

func (r Runtime) Run(ctx context.Context, request Request) (Response, error) {
	request = NormalizeRequest(request)
	for _, handler := range r.Handlers {
		if handler == nil {
			continue
		}
		response, handled, err := handler.HandleAgentRequest(ctx, request)
		if err != nil || handled {
			return NormalizeResponse(response), err
		}
	}
	if r.Fallback == nil {
		return Response{}, nil
	}
	response, _, err := r.Fallback.HandleAgentRequest(ctx, request)
	return NormalizeResponse(response), err
}

func NormalizeRequest(request Request) Request {
	request.Surface = Surface(strings.TrimSpace(string(request.Surface)))
	request.GuildID = strings.TrimSpace(request.GuildID)
	request.ChannelID = strings.TrimSpace(request.ChannelID)
	request.ActorUserID = strings.TrimSpace(request.ActorUserID)
	request.Text = strings.TrimSpace(request.Text)
	request.RawText = strings.TrimSpace(request.RawText)
	request.RoleIDs = append([]string(nil), request.RoleIDs...)
	return request
}

func NormalizeResponse(response Response) Response {
	response.Text = strings.TrimSpace(response.Text)
	response.Visibility = Visibility(strings.TrimSpace(string(response.Visibility)))
	return response
}
