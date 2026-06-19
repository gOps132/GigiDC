package agent

import (
	"context"
	"regexp"
	"strings"

	"github.com/gOps132/GigiDC/internal/llm"
	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
)

var answerCitationPattern = regexp.MustCompile(`\[S[0-9]+\]`)

type LLMAnswerer struct {
	Runtime         TextRuntime
	MaxOutputTokens int
	MaxInputChars   int
}

func (a LLMAnswerer) Answer(ctx context.Context, request Request, plan Plan, results []ToolResult) (Response, error) {
	if a.Runtime == nil {
		return Response{Text: formatToolResults(results)}, nil
	}
	input := a.answerPrompt(request, plan, results)
	if strings.TrimSpace(input) == "" {
		return Response{Text: formatToolResults(results)}, nil
	}
	generated, err := a.Runtime.GenerateText(ctx, llm.GenerateTextRequest{
		Owner:           llmprovider.Scope{OwnerType: llmprovider.OwnerGuild, GuildID: request.GuildID},
		Purpose:         llmprovider.PurposeChat,
		ActorUserID:     request.ActorUserID,
		GuildID:         request.GuildID,
		ChannelID:       request.ChannelID,
		Instructions:    llmAnswererInstructions(),
		Input:           input,
		MaxOutputTokens: a.maxOutputTokens(),
	})
	if err != nil {
		return Response{}, err
	}
	text := strings.TrimSpace(generated.Text)
	if text == "" {
		return Response{Text: formatToolResults(results)}, nil
	}
	if requiresEvidenceCitation(request, results) && !containsValidEvidenceCitation(text, request, results) {
		return Response{Text: formatToolResults(results)}, nil
	}
	return Response{Text: text, Visibility: VisibilityPublic}, nil
}

func llmAnswererInstructions() string {
	return "You are Gigi, a concise Discord assistant. Answer using only the provided tool results, context pack, and prior run context. Do not invent facts, counts, permissions, or actions. If you use context pack evidence, include citation labels like [S1]. If the user asks a follow-up, answer from prior/tool results when possible. Keep response short and useful."
}

func (a LLMAnswerer) answerPrompt(request Request, plan Plan, results []ToolResult) string {
	var b strings.Builder
	b.WriteString("User message:\n")
	b.WriteString(request.Text)
	b.WriteString("\n\nPlan intent:\n")
	b.WriteString(plan.Intent)
	if contextText := formatContextPack(request.ContextPack); contextText != "" {
		b.WriteString("\n\nContext pack:\n")
		b.WriteString(contextText)
	}
	if request.PriorRun != nil {
		b.WriteString("\n\nPrior run:\n")
		b.WriteString(formatRunSnapshot(*request.PriorRun, 1800))
	}
	b.WriteString("\n\nTool results:\n")
	for _, result := range results {
		b.WriteString("tool: ")
		b.WriteString(result.Name)
		if result.Summary != "" {
			b.WriteString("\nsummary: ")
			b.WriteString(result.Summary)
		}
		if len(result.Data) > 0 {
			b.WriteString("\ndata:\n")
			for key, value := range result.Data {
				b.WriteString("- ")
				b.WriteString(key)
				b.WriteString(": ")
				b.WriteString(value)
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	output := strings.TrimSpace(b.String())
	if max := a.maxInputChars(); max > 0 && len(output) > max {
		output = output[:max]
	}
	return output
}

func (a LLMAnswerer) maxOutputTokens() int {
	if a.MaxOutputTokens > 0 {
		return a.MaxOutputTokens
	}
	return 512
}

func (a LLMAnswerer) maxInputChars() int {
	if a.MaxInputChars > 0 {
		return a.MaxInputChars
	}
	return 6000
}

func requiresEvidenceCitation(request Request, results []ToolResult) bool {
	if len(request.ContextPack.Citations) > 0 {
		return true
	}
	for _, result := range results {
		if isMemoryEvidenceTool(result.Name) && strings.TrimSpace(result.Data["citation_labels"]) != "" {
			return true
		}
	}
	return false
}

func containsValidEvidenceCitation(text string, request Request, results []ToolResult) bool {
	valid := validEvidenceCitations(request, results)
	if len(valid) == 0 {
		return false
	}
	for _, label := range answerCitationPattern.FindAllString(text, -1) {
		if valid[strings.Trim(label, "[]")] {
			return true
		}
	}
	return false
}

func validEvidenceCitations(request Request, results []ToolResult) map[string]bool {
	valid := map[string]bool{}
	for _, citation := range request.ContextPack.Citations {
		if citation.Label != "" {
			valid[citation.Label] = true
		}
	}
	for _, result := range results {
		if !isMemoryEvidenceTool(result.Name) {
			continue
		}
		for _, label := range strings.Split(result.Data["citation_labels"], ",") {
			label = strings.TrimSpace(label)
			if label != "" {
				valid[label] = true
			}
		}
	}
	return valid
}

func isMemoryEvidenceTool(toolName string) bool {
	switch toolName {
	case ToolMemorySearch, ToolMemoryRecent:
		return true
	default:
		return false
	}
}
