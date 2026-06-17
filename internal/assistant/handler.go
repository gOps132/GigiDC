package assistant

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gOps132/GigiDC/internal/llm"
	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
)

const (
	SurfaceDM           Surface = "dm"
	SurfaceGuildMention Surface = "guild_mention"
)

const defaultInstructions = "You are Gigi, a concise Discord assistant. Answer the user's message directly. Do not claim you ran tools or changed server state."

type Surface string

type Message struct {
	Surface     Surface
	GuildID     string
	ChannelID   string
	ActorUserID string
	Text        string
}

type Response struct {
	Text string
}

type Runtime interface {
	GenerateText(ctx context.Context, req llm.GenerateTextRequest) (llm.TextResponse, error)
}

type Handler struct {
	Runtime          Runtime
	Recorder         ConversationRecorder
	Instructions     string
	MaxOutputTokens  int
	MaxResponseRunes int
}

func NewHandler(runtime Runtime) Handler {
	return Handler{Runtime: runtime}
}

func (h Handler) Reply(ctx context.Context, message Message) (Response, error) {
	message = normalizeMessage(message)
	if message.Text == "" {
		return Response{Text: "Say a little more and I can help."}, nil
	}
	if message.Surface != SurfaceGuildMention || message.GuildID == "" {
		return Response{Text: "Rich chat needs a server LLM profile first. Personal keys and DM reasoning are not enabled yet."}, nil
	}
	if h.Runtime == nil {
		return Response{}, fmt.Errorf("assistant runtime is required")
	}
	generated, err := h.Runtime.GenerateText(ctx, llm.GenerateTextRequest{
		Owner:           llmprovider.Scope{OwnerType: llmprovider.OwnerGuild, GuildID: message.GuildID},
		Purpose:         llmprovider.PurposeChat,
		ActorUserID:     message.ActorUserID,
		GuildID:         message.GuildID,
		ChannelID:       message.ChannelID,
		Instructions:    h.instructions(),
		Input:           message.Text,
		MaxOutputTokens: h.MaxOutputTokens,
	})
	if err != nil {
		return Response{}, err
	}
	text := strings.TrimSpace(generated.Text)
	if text == "" {
		return Response{Text: "I did not get a text response from the model."}, nil
	}
	text = truncateRunes(text, h.maxResponseRunes())
	if err := h.recordConversation(ctx, message, generated, text); err != nil {
		return Response{}, err
	}
	return Response{Text: text}, nil
}

func normalizeMessage(message Message) Message {
	message.Surface = Surface(strings.TrimSpace(string(message.Surface)))
	message.GuildID = strings.TrimSpace(message.GuildID)
	message.ChannelID = strings.TrimSpace(message.ChannelID)
	message.ActorUserID = strings.TrimSpace(message.ActorUserID)
	message.Text = strings.TrimSpace(message.Text)
	return message
}

func (h Handler) instructions() string {
	instructions := strings.TrimSpace(h.Instructions)
	if instructions == "" {
		return defaultInstructions
	}
	return instructions
}

func (h Handler) maxResponseRunes() int {
	if h.MaxResponseRunes > 0 {
		return h.MaxResponseRunes
	}
	return 1800
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:limit])) + "..."
}

func (h Handler) recordConversation(ctx context.Context, message Message, generated llm.TextResponse, responseText string) error {
	if h.Recorder == nil {
		return nil
	}
	base := ConversationTurn{
		RequestID:   generated.RequestID,
		Surface:     message.Surface,
		GuildID:     message.GuildID,
		ChannelID:   message.ChannelID,
		ActorUserID: message.ActorUserID,
		ProviderID:  generated.ProviderID,
		ModelID:     generated.ModelID,
	}
	userTurn := base
	userTurn.Role = TurnRoleUser
	userTurn.ContentChars = utf8.RuneCountInString(message.Text)
	if err := h.Recorder.RecordTurn(ctx, userTurn); err != nil {
		return err
	}
	assistantTurn := base
	assistantTurn.Role = TurnRoleAssistant
	assistantTurn.ContentChars = utf8.RuneCountInString(responseText)
	return h.Recorder.RecordTurn(ctx, assistantTurn)
}
