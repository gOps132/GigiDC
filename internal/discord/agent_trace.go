package discord

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/gOps132/GigiDC/internal/agent"
)

type AgentTraceReader interface {
	LastTrace(context.Context, agent.TraceQuery) (agent.TraceRun, bool, error)
}

func AgentCommands(reader AgentTraceReader, manager AgentRunManager, recorder AuditRecorder, configs ...AgentCommandConfig) []Command {
	cfg := AgentCommandConfig{}
	if len(configs) > 0 {
		cfg = configs[0]
	}
	return []Command{{
		Name:        "agent",
		Description: "Manage Gigi agent runs and diagnostics.",
		Options: append([]*discordgo.ApplicationCommandOption{{
			Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
			Name:        "trace",
			Description: "Inspect safe agent traces.",
			Options: []*discordgo.ApplicationCommandOption{{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "last",
				Description: "Show your last agent run in this channel.",
				Options: []*discordgo.ApplicationCommandOption{
					optionalStringOption("visibility", "Response visibility.", []*discordgo.ApplicationCommandOptionChoice{
						{Name: "private", Value: "private"},
						{Name: "public", Value: "public"},
					}),
					optionalStringOption("view", "Trace detail level.", []*discordgo.ApplicationCommandOptionChoice{
						{Name: "summary", Value: "summary"},
						{Name: "thinking", Value: "thinking"},
						{Name: "debug", Value: "debug"},
					}),
				},
			}, {
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "live",
				Description: "Run Gigi with a private live debug card.",
				Options: []*discordgo.ApplicationCommandOption{
					stringOption("prompt", "Guild-style message to debug.", nil),
				},
			}, {
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "mode",
				Description: "Toggle live debug for your guild mentions.",
				Options: []*discordgo.ApplicationCommandOption{
					stringOption("state", "Live debug mode.", []*discordgo.ApplicationCommandOptionChoice{
						{Name: "on", Value: "on"},
						{Name: "off", Value: "off"},
					}),
				},
			}},
		}, agentStatsOptions()}, agentRunOptions()...),
		Handle: agentHandler(reader, manager, recorder, cfg),
	}}
}

func agentHandler(reader AgentTraceReader, manager AgentRunManager, recorder AuditRecorder, cfg AgentCommandConfig) CommandHandler {
	traceHandler := agentTraceHandler(reader, cfg.Runtime, cfg.LiveDebugStore)
	statsHandler := agentStatsHandler(cfg.StatsReader, cfg.StatsAuthorizer, recorder, cfg.Clock, cfg.ReplyLatencyStore)
	runHandler := agentCommandHandler(manager, recorder)
	return func(ctx context.Context, interaction Interaction) (CommandResponse, error) {
		if len(interaction.Options) == 1 && interaction.Options[0].Name == "trace" {
			return traceHandler(ctx, interaction)
		}
		if len(interaction.Options) == 1 && interaction.Options[0].Name == "stats" {
			return statsHandler(ctx, interaction)
		}
		return runHandler(ctx, interaction)
	}
}

func agentTraceHandler(reader AgentTraceReader, runtime AgentRuntime, liveDebugStore AgentLiveDebugStore) CommandHandler {
	return func(ctx context.Context, interaction Interaction) (CommandResponse, error) {
		visibility := normalizeAgentTraceVisibility(agentTraceVisibility(interaction.Options))
		group, action, ok := agentTracePath(interaction)
		if ok && group == "trace" && action == "live" {
			return agentTraceLiveResponse(runtime, interaction), nil
		}
		if ok && group == "trace" && action == "mode" {
			return agentTraceModeResponse(ctx, liveDebugStore, interaction), nil
		}
		if reader == nil {
			return CommandResponse{Content: "Agent trace is not configured yet.", Ephemeral: visibility != "public"}, nil
		}
		if !ok || group != "trace" || action != "last" {
			return CommandResponse{Content: "Choose an agent trace action.", Ephemeral: true}, nil
		}
		run, ok, err := reader.LastTrace(ctx, agent.TraceQuery{
			GuildID:     interaction.GuildID,
			ChannelID:   interaction.ChannelID,
			ActorUserID: interaction.UserID,
		})
		if err != nil {
			return CommandResponse{Content: "Agent trace lookup failed.", Ephemeral: true}, nil
		}
		if !ok {
			return CommandResponse{Content: "No agent trace found for you in this channel.", Ephemeral: true}, nil
		}
		view := normalizeAgentTraceView(agentTraceView(interaction.Options))
		if view == "summary" {
			return CommandResponse{Content: formatAgentTrace(run), Ephemeral: visibility != "public"}, nil
		}
		return CommandResponse{Embeds: []*discordgo.MessageEmbed{formatAgentTraceEmbed(run, view)}, Ephemeral: visibility != "public"}, nil
	}
}

func agentTraceModeResponse(ctx context.Context, store AgentLiveDebugStore, interaction Interaction) CommandResponse {
	if store == nil {
		return CommandResponse{Content: "Agent live debug mode is not configured yet.", Ephemeral: true}
	}
	state := strings.TrimSpace(agentTraceModeState(interaction.Options))
	switch state {
	case "on":
		if err := store.SetLiveDebugEnabled(ctx, interaction.GuildID, interaction.UserID, true); err != nil {
			return CommandResponse{Content: "Agent live debug mode update failed.", Ephemeral: true}
		}
		return CommandResponse{Content: "Live debug is on for your @Gigi guild mentions. Debug cards are posted in-channel because Discord message events cannot create ephemeral messages.", Ephemeral: true}
	case "off":
		if err := store.SetLiveDebugEnabled(ctx, interaction.GuildID, interaction.UserID, false); err != nil {
			return CommandResponse{Content: "Agent live debug mode update failed.", Ephemeral: true}
		}
		return CommandResponse{Content: "Live debug is off for your @Gigi guild mentions.", Ephemeral: true}
	default:
		return CommandResponse{Content: "Choose live debug mode `on` or `off`.", Ephemeral: true}
	}
}

func agentTraceModeState(options []InteractionOption) string {
	if len(options) != 1 || len(options[0].Options) != 1 {
		return ""
	}
	return optionByName(options[0].Options[0].Options, "state")
}

func agentTracePath(interaction Interaction) (string, string, bool) {
	if len(interaction.Options) != 1 {
		return "", "", false
	}
	group := interaction.Options[0]
	if len(group.Options) != 1 {
		return group.Name, "", false
	}
	return group.Name, group.Options[0].Name, true
}

func agentTraceVisibility(options []InteractionOption) string {
	if len(options) != 1 || len(options[0].Options) != 1 {
		return "private"
	}
	return optionByName(options[0].Options[0].Options, "visibility")
}

func agentTraceView(options []InteractionOption) string {
	if len(options) != 1 || len(options[0].Options) != 1 {
		return "summary"
	}
	return optionByName(options[0].Options[0].Options, "view")
}

func normalizeAgentTraceVisibility(value string) string {
	switch strings.TrimSpace(value) {
	case "public":
		return "public"
	default:
		return "private"
	}
}

func normalizeAgentTraceView(value string) string {
	switch strings.TrimSpace(value) {
	case "thinking", "debug":
		return strings.TrimSpace(value)
	default:
		return "summary"
	}
}

func formatAgentTrace(run agent.TraceRun) string {
	lines := []string{fmt.Sprintf("Last agent run `%s`:", safeInline(run.RunID))}
	if run.Status != "" {
		lines = append(lines, fmt.Sprintf("status: `%s`", safeInline(run.Status)))
	}
	for _, event := range run.Events {
		lines = append(lines, formatAgentTraceEvent(event))
		if len(lines) >= 12 {
			break
		}
	}
	return strings.Join(lines, "\n")
}

func formatAgentTraceEvent(event agent.TraceEvent) string {
	parts := []string{"-"}
	if event.StepIndex > 0 {
		parts = append(parts, "`"+strconv.Itoa(event.StepIndex)+"`")
	}
	if event.Phase != "" {
		parts = append(parts, "`"+safeInline(event.Phase)+"`")
	}
	if event.Status != "" {
		parts = append(parts, safeInline(event.Status))
	}
	if event.Reason != "" {
		parts = append(parts, "reason=`"+safeInline(event.Reason)+"`")
	}
	if event.ToolName != "" {
		parts = append(parts, "tool=`"+safeInline(event.ToolName)+"`")
	}
	if event.ToolKind != "" {
		parts = append(parts, "kind=`"+safeInline(event.ToolKind)+"`")
	}
	if event.Capability != "" {
		parts = append(parts, "capability=`"+safeInline(event.Capability)+"`")
	}
	if event.RoutingMode != "" {
		parts = append(parts, "routing=`"+safeInline(event.RoutingMode)+"`")
	}
	if event.Phase == "tool" {
		if data := formatAgentTraceToolData(event.Details); data != "" {
			parts = append(parts, data)
		}
	}
	return strings.Join(parts, " ")
}

func formatAgentTraceEmbed(run agent.TraceRun, view string) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       "Gigi Agent Trace",
		Description: fmt.Sprintf("run `%s` status `%s`", safeInline(run.RunID), safeInline(run.Status)),
		Color:       agentTraceColor(run.Status),
	}
	if timeline := formatAgentTraceTimeline(run.Events); timeline != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Timeline", Value: timeline})
	}
	if planner := formatAgentTracePhase(run.Events, "plan", view); planner != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "LLM planning", Value: planner})
	}
	if tools := formatAgentTracePhase(run.Events, "tool", view); tools != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Tools", Value: tools})
	}
	if answer := formatAgentTracePhase(run.Events, "answer", view); answer != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Answer", Value: answer})
	}
	if view == "debug" {
		if details := formatAgentTraceDebugDetails(run.Events); details != "" {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Debug details", Value: details})
		}
	}
	return embed
}

func formatAgentTraceTimeline(events []agent.TraceEvent) string {
	lines := make([]string, 0, len(events))
	for _, event := range events {
		line := fmt.Sprintf("`%s` %s", safeInline(event.Phase), safeInline(event.Status))
		if event.Reason != "" {
			line += " reason=`" + safeInline(event.Reason) + "`"
		}
		if event.ToolName != "" {
			line += " tool=`" + safeInline(event.ToolName) + "`"
		}
		lines = append(lines, line)
		if len(lines) >= 8 {
			break
		}
	}
	return boundEmbedValue(strings.Join(lines, "\n"))
}

func formatAgentTracePhase(events []agent.TraceEvent, phase string, view string) string {
	lines := []string{}
	for _, event := range events {
		if event.Phase != phase {
			continue
		}
		switch phase {
		case "plan":
			line := fmt.Sprintf("intent=`%s` routing=`%s`", safeInline(event.Intent), safeInline(event.RoutingMode))
			if provider := event.Details["llm_provider"]; provider != "" {
				line += " model=`" + safeInline(event.Details["llm_model"]) + "` provider=`" + safeInline(provider) + "`"
			}
			if attempt := event.Details["llm_attempt"]; attempt != "" {
				line += " attempt=`" + safeInline(attempt) + "`"
			}
			lines = append(lines, line)
		case "tool":
			line := fmt.Sprintf("`%s` %s", safeInline(event.ToolName), safeInline(event.Status))
			if query := event.Details["arg_query"]; query != "" {
				line += " query=`" + safeInline(query) + "`"
			}
			if count := event.Details["result_count"]; count != "" {
				line += " results=`" + safeInline(count) + "`"
			}
			if data := formatAgentTraceToolData(event.Details); data != "" {
				line += " " + data
			}
			lines = append(lines, line)
		case "answer":
			line := "mode=`" + safeInline(event.Details["answer_mode"]) + "`"
			if reason := event.Details["fallback_reason"]; reason != "" {
				line += " fallback=`" + safeInline(reason) + "`"
			}
			if model := event.Details["llm_model"]; model != "" {
				line += " model=`" + safeInline(model) + "`"
			}
			lines = append(lines, line)
		}
		if view == "debug" {
			if preview := event.Details["llm_output_preview"]; preview != "" {
				lines = append(lines, "llm_preview: "+safeBlockLine(preview))
			}
			if summary := event.Details["result_summary"]; summary != "" {
				lines = append(lines, "result: "+safeBlockLine(summary))
			}
			if preview := event.Details["answer_preview"]; preview != "" {
				lines = append(lines, "answer: "+safeBlockLine(preview))
			}
		}
	}
	return boundEmbedValue(strings.Join(lines, "\n"))
}

func formatAgentTraceToolData(details map[string]string) string {
	if len(details) == 0 {
		return ""
	}
	keys := make([]string, 0, len(details))
	for key := range details {
		if strings.HasPrefix(key, "result_data_") {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(details[key])
		if value == "" {
			continue
		}
		label := strings.TrimPrefix(key, "result_data_")
		parts = append(parts, "data."+safeInline(label)+"=`"+safeInline(value)+"`")
		if len(parts) >= 4 {
			break
		}
	}
	return strings.Join(parts, " ")
}

func formatAgentTraceDebugDetails(events []agent.TraceEvent) string {
	lines := []string{}
	for _, event := range events {
		for key, value := range event.Details {
			switch key {
			case "llm_output_preview", "result_summary", "answer_preview":
				continue
			}
			lines = append(lines, fmt.Sprintf("`%s.%s`=`%s`", safeInline(event.Phase), safeInline(key), safeInline(value)))
			if len(lines) >= 14 {
				return boundEmbedValue(strings.Join(lines, "\n"))
			}
		}
	}
	return boundEmbedValue(strings.Join(lines, "\n"))
}

func agentTraceColor(status string) int {
	switch strings.TrimSpace(status) {
	case "failed", "denied", "canceled":
		return 0xD83A34
	case "succeeded":
		return 0x2E7D32
	default:
		return 0x5865F2
	}
}

func boundEmbedValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) > 1000 {
		return value[:997] + "..."
	}
	return value
}

func safeBlockLine(value string) string {
	value = strings.TrimSpace(strings.NewReplacer("\n", " ", "\r", " ", "`", "'").Replace(value))
	if len(value) > 220 {
		value = value[:217] + "..."
	}
	return value
}

func agentTraceLiveResponse(runtime AgentRuntime, interaction Interaction) CommandResponse {
	if runtime == nil {
		return CommandResponse{Content: "Agent runtime is not configured yet.", Ephemeral: true}
	}
	prompt := strings.TrimSpace(agentTracePrompt(interaction.Options))
	if prompt == "" {
		return CommandResponse{Content: "Prompt is required.", Ephemeral: true}
	}
	return CommandResponse{
		Ephemeral: true,
		Deferred:  true,
		AfterRespond: func(ctx context.Context, editor InteractionResponseEditor) {
			runAgentTraceLive(ctx, runtime, interaction, prompt, editor)
		},
	}
}

func agentTracePrompt(options []InteractionOption) string {
	if len(options) != 1 || len(options[0].Options) != 1 {
		return ""
	}
	return optionByName(options[0].Options[0].Options, "prompt")
}

type liveAgentTraceSink struct {
	mu     sync.Mutex
	editor InteractionResponseEditor
	run    agent.TraceRun
}

func runAgentTraceLive(ctx context.Context, runtime AgentRuntime, interaction Interaction, prompt string, editor InteractionResponseEditor) {
	sink := &liveAgentTraceSink{editor: editor}
	sink.update(ctx, "running", "")
	response, err := runtime.Run(ctx, agent.Request{
		Surface:          agent.SurfaceGuildMention,
		GuildID:          interaction.GuildID,
		ChannelID:        interaction.ChannelID,
		ActorUserID:      interaction.UserID,
		RoleIDs:          interaction.RoleIDs,
		HasAdministrator: interaction.HasAdministrator,
		ContextScope:     "channel-auto",
		Text:             prompt,
		RawText:          prompt,
		TraceSink:        sink,
	})
	if err != nil {
		sink.update(ctx, "failed", "Agent debug run failed.")
		return
	}
	sink.mu.Lock()
	if response.RunID != "" {
		sink.run.RunID = response.RunID
	}
	if response.RunStatus != "" {
		sink.run.Status = string(response.RunStatus)
	}
	sink.mu.Unlock()
	sink.update(ctx, string(response.RunStatus), response.Text)
}

func (s *liveAgentTraceSink) RecordTraceEvent(ctx context.Context, request agent.Request, event agent.TraceEvent) error {
	s.mu.Lock()
	if s.run.RunID == "" {
		s.run.RunID = event.RunID
	}
	s.run.GuildID = request.GuildID
	s.run.ChannelID = request.ChannelID
	s.run.ActorUserID = request.ActorUserID
	s.run.Surface = string(request.Surface)
	s.run.Status = event.Status
	s.run.Events = append(s.run.Events, event)
	s.mu.Unlock()
	s.update(ctx, event.Status, "")
	return nil
}

func (s *liveAgentTraceSink) update(ctx context.Context, status string, answer string) {
	if s == nil || s.editor == nil {
		return
	}
	s.mu.Lock()
	run := s.run
	if run.Status == "" {
		run.Status = status
	}
	s.mu.Unlock()
	embed := formatAgentTraceEmbed(run, "debug")
	if strings.TrimSpace(answer) != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Final answer", Value: boundEmbedValue(answer)})
	}
	content := "Live Gigi debug"
	embeds := []*discordgo.MessageEmbed{embed}
	_ = s.editor.EditInteractionResponse(ctx, &discordgo.WebhookEdit{
		Content: &content,
		Embeds:  &embeds,
	})
}
