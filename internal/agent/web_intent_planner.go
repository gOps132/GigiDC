package agent

import (
	"context"
	"regexp"
	"sort"
	"strings"
)

var webURLPattern = regexp.MustCompile(`https?://[^\s<>()]+`)

// WebIntentPlanner handles high-confidence web/tool mentions before LLM routing.
type WebIntentPlanner struct{}

func (p WebIntentPlanner) Plan(ctx context.Context, request Request, specs []ToolSpec) (Plan, bool, error) {
	text := strings.TrimSpace(request.Text)
	if request.Surface != SurfaceGuildMention || request.GuildID == "" || text == "" {
		return Plan{}, false, nil
	}
	if isToolListQuestion(text) {
		return Plan{
			Intent:             "tools.list",
			ClarifyingQuestion: availableToolsText(specs),
		}, true, nil
	}
	if hasToolSpec(specs, ToolWebFetch) {
		if targetURL, ok := webFetchRequest(text); ok {
			return Plan{
				Intent:    ToolWebFetch,
				ToolCalls: []ToolCall{{Name: ToolWebFetch, Args: map[string]string{"url": targetURL}}},
			}, true, nil
		}
	}
	if hasToolSpec(specs, ToolWebSearch) {
		if query, ok := webSearchRequest(text); ok {
			return Plan{
				Intent:    ToolWebSearch,
				ToolCalls: []ToolCall{{Name: ToolWebSearch, Args: map[string]string{"query": query}}},
			}, true, nil
		}
	}
	return Plan{}, false, nil
}

func webFetchRequest(text string) (string, bool) {
	match := webURLPattern.FindString(text)
	if match == "" {
		return "", false
	}
	lower := strings.ToLower(text)
	for _, marker := range []string{"fetch", "read", "summarize", "summary", "inspect", "open", "page", "url"} {
		if strings.Contains(lower, marker) {
			return strings.TrimRight(match, ".,;:!?"), true
		}
	}
	return "", false
}

func webSearchRequest(text string) (string, bool) {
	lower := strings.ToLower(text)
	explicit := []string{
		"search the web",
		"search web",
		"web search",
		"search online",
		"look up online",
		"lookup online",
		"google",
	}
	current := []string{
		"latest ",
		"current ",
		"recent ",
		"today",
		"release notes",
	}
	if !containsAny(lower, explicit) && !containsAny(lower, current) {
		return "", false
	}
	query := stripSearchPrefix(text)
	if query == "" {
		return "", false
	}
	return query, true
}

func stripSearchPrefix(text string) string {
	query := strings.TrimSpace(text)
	lower := strings.ToLower(query)
	prefixes := []string{
		"search the web for ",
		"search web for ",
		"web search for ",
		"search online for ",
		"look up online ",
		"lookup online ",
		"google ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(query[len(prefix):])
		}
	}
	return query
}

func isToolListQuestion(text string) bool {
	lower := strings.ToLower(text)
	return (strings.Contains(lower, "what tools") || strings.Contains(lower, "which tools") || strings.Contains(lower, "tools can you use")) &&
		(strings.Contains(lower, "you") || strings.Contains(lower, "gigi"))
}

func availableToolsText(specs []ToolSpec) string {
	names := make([]string, 0, len(specs))
	for _, spec := range specs {
		spec = NormalizeToolSpec(spec)
		if spec.Name != "" {
			names = append(names, "`"+spec.Name+"`")
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "No agent tools registered."
	}
	return "Available agent tools: " + strings.Join(names, ", ") + "."
}

func hasToolSpec(specs []ToolSpec, name string) bool {
	for _, spec := range specs {
		if NormalizeToolSpec(spec).Name == name {
			return true
		}
	}
	return false
}

func containsAny(value string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
