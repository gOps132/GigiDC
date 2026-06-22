package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gOps132/GigiDC/internal/llm"
	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
	"github.com/gOps132/GigiDC/internal/plugins"
)

type SemanticPluginInput struct {
	GuildID     string
	ChannelID   string
	ActorUserID string
	Text        string
	Manifests   []plugins.Manifest
}

type SemanticPluginPlanner struct {
	Runtime         Runtime
	MaxOutputTokens int
}

type semanticPluginProposal struct {
	Intent    string `json:"intent"`
	PluginID  string `json:"plugin_id"`
	Trigger   string `json:"trigger"`
	Arguments string `json:"arguments"`
}

func (p SemanticPluginPlanner) Plan(ctx context.Context, input SemanticPluginInput) (plugins.CommandPlan, bool, error) {
	input = normalizeSemanticPluginInput(input)
	if input.GuildID == "" || input.ActorUserID == "" || input.Text == "" || len(input.Manifests) == 0 {
		return plugins.CommandPlan{}, false, nil
	}
	for _, candidate := range semanticTextCandidates(input.Text) {
		if plan, ok := plugins.PlanCommand(input.Manifests, "guild_text", candidate); ok {
			return plan, true, nil
		}
	}
	if p.Runtime == nil {
		return plugins.CommandPlan{}, false, fmt.Errorf("semantic plugin runtime is required")
	}
	generated, err := p.Runtime.GenerateText(ctx, llm.GenerateTextRequest{
		Owner:           llmprovider.Scope{OwnerType: llmprovider.OwnerGuild, GuildID: input.GuildID},
		Purpose:         llmprovider.PurposeRouting,
		ActorUserID:     input.ActorUserID,
		GuildID:         input.GuildID,
		ChannelID:       input.ChannelID,
		Instructions:    semanticPluginInstructions(),
		Input:           semanticPluginPrompt(input),
		MaxOutputTokens: p.maxOutputTokens(),
	})
	if err != nil {
		return plugins.CommandPlan{}, false, err
	}
	proposal, ok := parseSemanticPluginProposal(generated.Text)
	if !ok {
		return plugins.CommandPlan{}, false, nil
	}
	plan, ok := plugins.PlanCommandFromTrigger(input.Manifests, "guild_text", proposal.PluginID, proposal.Trigger, proposal.Arguments)
	return plan, ok, nil
}

func normalizeSemanticPluginInput(input SemanticPluginInput) SemanticPluginInput {
	input.GuildID = strings.TrimSpace(input.GuildID)
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Text = strings.TrimSpace(input.Text)
	return input
}

func (p SemanticPluginPlanner) maxOutputTokens() int {
	if p.MaxOutputTokens > 0 {
		return p.MaxOutputTokens
	}
	return 256
}

func semanticPluginInstructions() string {
	return "You map a Discord message to one enabled external app prefix trigger only when the user's primary intent is to operate that external app. Return only JSON with intent, plugin_id, trigger, and arguments. Use intent \"plugin_command\" for a valid external app command. Return {} for Gigi meta questions, tool/capability questions, general chat, memory questions, or web/public fact lookup requests. Do not invent plugin IDs or triggers."
}

func semanticPluginPrompt(input SemanticPluginInput) string {
	var b strings.Builder
	b.WriteString("User message:\n")
	b.WriteString(input.Text)
	b.WriteString("\n\nEnabled plugins:\n")
	for _, manifest := range input.Manifests {
		b.WriteString("- plugin_id: ")
		b.WriteString(strings.TrimSpace(manifest.ID))
		b.WriteString("\n  name: ")
		b.WriteString(strings.TrimSpace(manifest.Name))
		b.WriteString("\n  triggers:\n")
		for _, trigger := range manifest.Triggers {
			if strings.TrimSpace(trigger.Kind) != "prefix" {
				continue
			}
			b.WriteString("    - ")
			b.WriteString(strings.TrimSpace(trigger.Value))
			if len(trigger.Aliases) > 0 {
				b.WriteString(" aliases: ")
				b.WriteString(strings.Join(trigger.Aliases, ", "))
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\nReturn JSON like {\"intent\":\"plugin_command\",\"plugin_id\":\"example\",\"trigger\":\"!play\",\"arguments\":\"song\"}. Return {} when the message is not asking to operate one enabled external app.")
	return b.String()
}

func semanticTextCandidates(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	candidates := []string{text}
	lower := strings.ToLower(text)
	for _, prefix := range []string{
		"please ",
		"pls ",
		"can you ",
		"could you ",
		"would you ",
		"gigi please ",
		"gigi ",
	} {
		if strings.HasPrefix(lower, prefix) {
			trimmed := strings.TrimSpace(text[len(prefix):])
			if trimmed != "" && !containsString(candidates, trimmed) {
				candidates = append(candidates, trimmed)
			}
		}
	}
	return candidates
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func parseSemanticPluginProposal(value string) (semanticPluginProposal, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return semanticPluginProposal{}, false
	}
	start := strings.Index(value, "{")
	end := strings.LastIndex(value, "}")
	if start < 0 || end < start {
		return semanticPluginProposal{}, false
	}
	var proposal semanticPluginProposal
	if err := json.Unmarshal([]byte(value[start:end+1]), &proposal); err != nil {
		return semanticPluginProposal{}, false
	}
	proposal.PluginID = strings.TrimSpace(proposal.PluginID)
	proposal.Trigger = strings.TrimSpace(proposal.Trigger)
	proposal.Arguments = strings.TrimSpace(proposal.Arguments)
	proposal.Intent = strings.TrimSpace(proposal.Intent)
	if proposal.Intent != "plugin_command" || proposal.PluginID == "" || proposal.Trigger == "" {
		return semanticPluginProposal{}, false
	}
	return proposal, true
}
