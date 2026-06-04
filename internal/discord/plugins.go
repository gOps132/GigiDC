package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/plugins"
)

type PluginCatalogManager interface {
	ApprovedManifests(ctx context.Context) ([]plugins.Manifest, error)
	UpsertApprovedManifest(ctx context.Context, manifest plugins.Manifest, actorID string) error
	EnableForGuild(ctx context.Context, guildID string, pluginID string, version string, actorID string) error
	DisableForGuild(ctx context.Context, guildID string, pluginID string, actorID string) error
	EnabledForGuild(ctx context.Context, guildID string) ([]plugins.Manifest, error)
}

type PluginManifestFetcher interface {
	Fetch(ctx context.Context, manifestURL string) (plugins.Manifest, error)
	FetchAttachment(ctx context.Context, attachment plugins.AttachmentSource) (plugins.Manifest, error)
}

func PluginCommands(manager PluginCatalogManager, fetcher PluginManifestFetcher, recorder AuditRecorder) []Command {
	return []Command{{
		Name:               "plugins",
		Description:        "Manage approved Gigi plugin manifests.",
		RequiredCapability: capability.Capability("plugin.install"),
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "list",
				Description: "List approved plugin manifests.",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "import-manifest",
				Description: "Import and approve a plugin manifest from HTTPS.",
				Options: []*discordgo.ApplicationCommandOption{
					stringOption("url", "HTTPS URL for gigi-plugin.json.", nil),
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "import-file",
				Description: "Import and approve an uploaded plugin manifest JSON file.",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionAttachment,
						Name:        "attachment",
						Description: "gigi-plugin.json file.",
						Required:    true,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "enable",
				Description: "Enable an approved plugin for this server.",
				Options: []*discordgo.ApplicationCommandOption{
					stringOption("plugin", "Plugin id.", nil),
					stringOption("version", "Plugin version.", nil),
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "disable",
				Description: "Disable a plugin for this server.",
				Options: []*discordgo.ApplicationCommandOption{
					stringOption("plugin", "Plugin id.", nil),
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "enabled",
				Description: "List plugins enabled for this server.",
			},
		},
		Handle: pluginHandler(manager, fetcher, recorder),
	}}
}

func pluginHandler(manager PluginCatalogManager, fetcher PluginManifestFetcher, recorder AuditRecorder) CommandHandler {
	return func(ctx context.Context, interaction Interaction) (CommandResponse, error) {
		if manager == nil {
			return CommandResponse{}, fmt.Errorf("plugin catalog manager is required")
		}
		request, err := parsePluginRequest(interaction)
		if err != nil {
			_ = recordPluginAction(ctx, recorder, interaction, request, audit.StatusFailed, err)
			return CommandResponse{Content: err.Error(), Ephemeral: true}, nil
		}

		response, err := executePluginRequest(ctx, manager, fetcher, interaction, &request)
		if err != nil {
			_ = recordPluginAction(ctx, recorder, interaction, request, audit.StatusFailed, err)
			return CommandResponse{Content: cleanPluginError(err), Ephemeral: true}, nil
		}
		if shouldAuditPluginAction(request.Action) {
			if err := recordPluginAction(ctx, recorder, interaction, request, audit.StatusSucceeded, nil); err != nil {
				return CommandResponse{}, err
			}
		}
		return CommandResponse{Content: response, Ephemeral: true}, nil
	}
}

type pluginRequest struct {
	Action       string
	URL          string
	AttachmentID string
	Attachment   InteractionAttachment
	Plugin       string
	Version      string
}

func parsePluginRequest(interaction Interaction) (pluginRequest, error) {
	if strings.TrimSpace(interaction.GuildID) == "" {
		return pluginRequest{}, fmt.Errorf("Plugins can only be managed inside a Discord server.")
	}
	if len(interaction.Options) != 1 {
		return pluginRequest{}, fmt.Errorf("Choose one plugins action.")
	}
	action := interaction.Options[0]
	request := pluginRequest{Action: action.Name}
	switch request.Action {
	case "list", "enabled":
		return request, nil
	case "import-manifest":
		request.URL = strings.TrimSpace(optionByName(action.Options, "url"))
		if request.URL == "" {
			return request, fmt.Errorf("Manifest URL is required.")
		}
		return request, nil
	case "import-file":
		request.AttachmentID = strings.TrimSpace(optionByName(action.Options, "attachment"))
		if request.AttachmentID == "" {
			return request, fmt.Errorf("Manifest attachment is required.")
		}
		attachment, ok := interaction.Attachments[request.AttachmentID]
		if !ok {
			return request, fmt.Errorf("Manifest attachment could not be resolved.")
		}
		request.Attachment = attachment
		return request, nil
	case "enable":
		request.Plugin = strings.TrimSpace(optionByName(action.Options, "plugin"))
		request.Version = strings.TrimSpace(optionByName(action.Options, "version"))
		if request.Plugin == "" || request.Version == "" {
			return request, fmt.Errorf("Plugin id and version are required.")
		}
		return request, nil
	case "disable":
		request.Plugin = strings.TrimSpace(optionByName(action.Options, "plugin"))
		if request.Plugin == "" {
			return request, fmt.Errorf("Plugin id is required.")
		}
		return request, nil
	default:
		return request, fmt.Errorf("Unsupported plugins action.")
	}
}

func executePluginRequest(ctx context.Context, manager PluginCatalogManager, fetcher PluginManifestFetcher, interaction Interaction, request *pluginRequest) (string, error) {
	switch request.Action {
	case "list":
		manifests, err := manager.ApprovedManifests(ctx)
		if err != nil {
			return "", err
		}
		return formatPluginList("Approved plugins", manifests), nil
	case "enabled":
		manifests, err := manager.EnabledForGuild(ctx, interaction.GuildID)
		if err != nil {
			return "", err
		}
		return formatPluginList("Enabled plugins", manifests), nil
	case "import-manifest":
		if fetcher == nil {
			return "", fmt.Errorf("plugin manifest fetcher is required")
		}
		manifest, err := fetcher.Fetch(ctx, request.URL)
		if err != nil {
			return "", err
		}
		if err := manager.UpsertApprovedManifest(ctx, manifest, interaction.UserID); err != nil {
			return "", err
		}
		request.Plugin = manifest.ID
		request.Version = manifest.Version
		return fmt.Sprintf("Imported plugin `%s` (`%s@%s`).", safeInline(manifest.Name), safeInline(manifest.ID), safeInline(manifest.Version)), nil
	case "import-file":
		if fetcher == nil {
			return "", fmt.Errorf("plugin manifest fetcher is required")
		}
		manifest, err := fetcher.FetchAttachment(ctx, plugins.AttachmentSource{
			ID:          request.Attachment.ID,
			URL:         request.Attachment.URL,
			Filename:    request.Attachment.Filename,
			ContentType: request.Attachment.ContentType,
			Size:        request.Attachment.Size,
		})
		if err != nil {
			return "", err
		}
		if err := manager.UpsertApprovedManifest(ctx, manifest, interaction.UserID); err != nil {
			return "", err
		}
		request.Plugin = manifest.ID
		request.Version = manifest.Version
		return fmt.Sprintf("Imported plugin `%s` (`%s@%s`) from uploaded file.", safeInline(manifest.Name), safeInline(manifest.ID), safeInline(manifest.Version)), nil
	case "enable":
		if err := manager.EnableForGuild(ctx, interaction.GuildID, request.Plugin, request.Version, interaction.UserID); err != nil {
			return "", err
		}
		return fmt.Sprintf("Enabled plugin `%s@%s` for this server.", safeInline(request.Plugin), safeInline(request.Version)), nil
	case "disable":
		if err := manager.DisableForGuild(ctx, interaction.GuildID, request.Plugin, interaction.UserID); err != nil {
			return "", err
		}
		return fmt.Sprintf("Disabled plugin `%s` for this server.", safeInline(request.Plugin)), nil
	default:
		return "", fmt.Errorf("unsupported plugins action")
	}
}

func formatPluginList(title string, manifests []plugins.Manifest) string {
	if len(manifests) == 0 {
		return title + ": none."
	}
	limit := len(manifests)
	if limit > 10 {
		limit = 10
	}
	lines := []string{title + ":"}
	for _, manifest := range manifests[:limit] {
		lines = append(lines, fmt.Sprintf("- `%s@%s` - %s", safeInline(manifest.ID), safeInline(manifest.Version), safeInline(manifest.Name)))
	}
	if len(manifests) > limit {
		lines = append(lines, fmt.Sprintf("...and %d more.", len(manifests)-limit))
	}
	return strings.Join(lines, "\n")
}

func cleanPluginError(err error) string {
	if err == nil {
		return "Plugin command failed."
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "HTTPS"):
		return "Manifest URL must be HTTPS."
	case strings.Contains(message, "user info") || strings.Contains(message, "query") || strings.Contains(message, "fragment"):
		return "Manifest URL must not include user info, query, or fragment data."
	case strings.Contains(message, "byte limit"):
		return "Manifest is too large."
	case strings.Contains(message, "JSON file"):
		return "Manifest attachment must be a JSON file."
	case strings.Contains(message, "attachment"):
		return "Manifest attachment could not be imported."
	case strings.Contains(message, "not found"):
		return "Plugin or version was not found."
	default:
		return "Plugin command failed."
	}
}

func recordPluginAction(ctx context.Context, recorder AuditRecorder, interaction Interaction, request pluginRequest, status audit.Status, err error) error {
	if recorder == nil || strings.TrimSpace(interaction.UserID) == "" || !shouldAuditPluginAction(request.Action) {
		return nil
	}
	reason := ""
	if err != nil {
		reason = "plugin_action_failed"
	}
	metadata := map[string]string{
		"command": interaction.Name,
		"action":  request.Action,
	}
	if request.Plugin != "" {
		metadata["plugin_id"] = request.Plugin
	}
	if request.Version != "" {
		metadata["version"] = request.Version
	}
	return recorder.Record(ctx, audit.Event{
		Kind:     "discord.plugins.change",
		GuildID:  interaction.GuildID,
		ActorID:  interaction.UserID,
		Status:   status,
		Reason:   reason,
		Metadata: metadata,
	})
}

func shouldAuditPluginAction(action string) bool {
	switch action {
	case "import-manifest", "import-file", "enable", "disable":
		return true
	default:
		return false
	}
}

func safeInline(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "`", "'")
}
