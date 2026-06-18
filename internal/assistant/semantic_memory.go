package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/gOps132/GigiDC/internal/llm"
	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
)

type MemoryIntent string

const (
	MemoryIntentCount  MemoryIntent = "count"
	MemoryIntentSearch MemoryIntent = "search"
)

type SemanticMemoryInput struct {
	GuildID     string
	ChannelID   string
	ActorUserID string
	Text        string
}

type MemoryPlan struct {
	Intent       MemoryIntent
	TargetUserID string
	Text         string
	Query        string
	Scope        string
	Limit        int
}

type SemanticMemoryPlanner struct {
	Runtime         Runtime
	MaxOutputTokens int
}

type semanticMemoryProposal struct {
	Intent       string `json:"intent"`
	TargetUserID string `json:"target_user_id"`
	Text         string `json:"text"`
	Query        string `json:"query"`
	Scope        string `json:"scope"`
	Limit        int    `json:"limit"`
}

func (p SemanticMemoryPlanner) Plan(ctx context.Context, input SemanticMemoryInput) (MemoryPlan, bool, error) {
	input = normalizeSemanticMemoryInput(input)
	if input.GuildID == "" || input.ChannelID == "" || input.ActorUserID == "" || input.Text == "" {
		return MemoryPlan{}, false, nil
	}
	if p.Runtime == nil {
		return MemoryPlan{}, false, fmt.Errorf("semantic memory runtime is required")
	}
	generated, err := p.Runtime.GenerateText(ctx, llm.GenerateTextRequest{
		Owner:           llmprovider.Scope{OwnerType: llmprovider.OwnerGuild, GuildID: input.GuildID},
		Purpose:         llmprovider.PurposeRouting,
		ActorUserID:     input.ActorUserID,
		GuildID:         input.GuildID,
		ChannelID:       input.ChannelID,
		Instructions:    semanticMemoryInstructions(),
		Input:           semanticMemoryPrompt(input),
		MaxOutputTokens: p.maxOutputTokens(),
	})
	if err != nil {
		return MemoryPlan{}, false, err
	}
	return parseSemanticMemoryPlanForInput(generated.Text, input)
}

func normalizeSemanticMemoryInput(input SemanticMemoryInput) SemanticMemoryInput {
	input.GuildID = strings.TrimSpace(input.GuildID)
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Text = strings.TrimSpace(input.Text)
	return input
}

func (p SemanticMemoryPlanner) maxOutputTokens() int {
	if p.MaxOutputTokens > 0 {
		return p.MaxOutputTokens
	}
	return 192
}

func semanticMemoryInstructions() string {
	return "You plan Gigi memory tools for one Discord guild mention. Return only JSON. Allowed intents: count, search. Use only this-channel scope. For count, text is required. Include target_user_id only when the user asks about a specific mentioned user, and only use an ID from Mentioned user IDs. Omit target_user_id for channel-wide counts. For search, query is required. If message is not a memory count/search request, return {}. Do not answer the user."
}

func semanticMemoryPrompt(input SemanticMemoryInput) string {
	mentions := extractMentionedUserIDs(input.Text)
	mentionList := "none"
	if len(mentions) > 0 {
		mentionList = strings.Join(mentions, ", ")
	}
	return "User message:\n" + input.Text + "\n\nMentioned user IDs: " + mentionList + "\n\nReturn one JSON object like {\"intent\":\"count\",\"target_user_id\":\"123\",\"text\":\"postgres\",\"scope\":\"this-channel\"}, {\"intent\":\"count\",\"text\":\"postgres\",\"scope\":\"this-channel\"}, or {\"intent\":\"search\",\"query\":\"postgres\",\"limit\":5,\"scope\":\"this-channel\"}."
}

func parseSemanticMemoryPlan(value string) (MemoryPlan, bool, error) {
	return parseSemanticMemoryPlanForInput(value, SemanticMemoryInput{})
}

func parseSemanticMemoryPlanForInput(value string, input SemanticMemoryInput) (MemoryPlan, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return MemoryPlan{}, false, nil
	}
	start := strings.Index(value, "{")
	end := strings.LastIndex(value, "}")
	if start < 0 || end < start {
		return MemoryPlan{}, false, nil
	}
	var proposal semanticMemoryProposal
	if err := json.Unmarshal([]byte(value[start:end+1]), &proposal); err != nil {
		return MemoryPlan{}, false, nil
	}
	plan := MemoryPlan{
		Intent:       MemoryIntent(strings.TrimSpace(proposal.Intent)),
		TargetUserID: strings.TrimSpace(proposal.TargetUserID),
		Text:         strings.TrimSpace(proposal.Text),
		Query:        strings.TrimSpace(proposal.Query),
		Scope:        strings.TrimSpace(proposal.Scope),
		Limit:        proposal.Limit,
	}
	if plan.Scope == "" {
		plan.Scope = "this-channel"
	}
	if plan.Limit == 0 {
		plan.Limit = 5
	}
	if plan.Limit < 1 {
		plan.Limit = 1
	}
	if plan.Limit > 25 {
		plan.Limit = 25
	}
	if plan.Scope != "this-channel" {
		return MemoryPlan{}, false, nil
	}
	switch plan.Intent {
	case MemoryIntentCount:
		if strings.TrimSpace(plan.Text) == "" {
			return MemoryPlan{}, false, nil
		}
		plan.TargetUserID = validMentionedTarget(plan.TargetUserID, input.Text)
	case MemoryIntentSearch:
		if strings.TrimSpace(plan.Query) == "" {
			return MemoryPlan{}, false, nil
		}
	default:
		return MemoryPlan{}, false, nil
	}
	return plan, true, nil
}

var discordUserMentionPattern = regexp.MustCompile(`<@!?([0-9]+)>`)

func extractMentionedUserIDs(text string) []string {
	matches := discordUserMentionPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 || match[1] == "" {
			continue
		}
		if _, ok := seen[match[1]]; ok {
			continue
		}
		seen[match[1]] = struct{}{}
		ids = append(ids, match[1])
	}
	return ids
}

func validMentionedTarget(targetUserID string, text string) string {
	targetUserID = strings.TrimSpace(targetUserID)
	if targetUserID == "" {
		return ""
	}
	for _, mentionedUserID := range extractMentionedUserIDs(text) {
		if targetUserID == mentionedUserID {
			return targetUserID
		}
	}
	return ""
}
