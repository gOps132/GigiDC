package agent

import (
	"context"
	"fmt"

	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
)

const ToolLLMUsageGuild = "llm.usage.guild"

type UsageReporter interface {
	GuildUsageSummary(ctx context.Context, guildID string) (llmprovider.UsageSummary, error)
}

type LLMUsageGuildTool struct {
	Reporter UsageReporter
}

func (t LLMUsageGuildTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        ToolLLMUsageGuild,
		Description: "Summarize recorded LLM token usage for this guild.",
		Kind:        ToolKindRead,
		Capability:  "llm.provider.select",
	}
}

func (t LLMUsageGuildTool) Execute(ctx context.Context, request Request, call ToolCall) (ToolResult, error) {
	if request.Surface != SurfaceGuildMention || request.GuildID == "" {
		return ToolResult{}, fmt.Errorf("guild usage context is required")
	}
	if t.Reporter == nil {
		return ToolResult{}, fmt.Errorf("usage reporter is required")
	}
	summary, err := t.Reporter.GuildUsageSummary(ctx, request.GuildID)
	if err != nil {
		return ToolResult{}, err
	}
	totalTokens := summary.InputTokens + summary.OutputTokens
	return ToolResult{
		Name:    ToolLLMUsageGuild,
		Summary: formatLLMUsageSummary(summary),
		Data: map[string]string{
			"input_tokens":  fmt.Sprintf("%d", summary.InputTokens),
			"output_tokens": fmt.Sprintf("%d", summary.OutputTokens),
			"total_tokens":  fmt.Sprintf("%d", totalTokens),
			"total_events":  fmt.Sprintf("%d", summary.TotalEvents),
			"failed_events": fmt.Sprintf("%d", summary.FailedEvents),
		},
	}, nil
}

func formatLLMUsageSummary(summary llmprovider.UsageSummary) string {
	if summary.TotalEvents == 0 {
		return "LLM usage for this server: no recorded usage."
	}
	totalTokens := summary.InputTokens + summary.OutputTokens
	requestWord := "requests"
	if summary.TotalEvents == 1 {
		requestWord = "request"
	}
	failedPart := ""
	if summary.FailedEvents > 0 {
		failedPart = fmt.Sprintf("; %d failed", summary.FailedEvents)
	}
	return fmt.Sprintf("LLM usage for this server: %d tokens (%d input, %d output) across %d %s%s.",
		totalTokens,
		summary.InputTokens,
		summary.OutputTokens,
		summary.TotalEvents,
		requestWord,
		failedPart,
	)
}
