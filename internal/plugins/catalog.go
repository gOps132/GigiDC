package plugins

import (
	"fmt"
	"strings"
)

type StaticCatalog struct {
	byApplicationID map[string]Manifest
	byBotUserID     map[string]Manifest
}

func NewStaticCatalog(manifests ...Manifest) (StaticCatalog, error) {
	catalog := StaticCatalog{
		byApplicationID: make(map[string]Manifest),
		byBotUserID:     make(map[string]Manifest),
	}
	for _, manifest := range manifests {
		if err := manifest.Validate(); err != nil {
			return StaticCatalog{}, err
		}
		if appID := strings.TrimSpace(manifest.DiscordApplicationID); appID != "" {
			if existing, ok := catalog.byApplicationID[appID]; ok {
				return StaticCatalog{}, fmt.Errorf("duplicate Discord application ID %q for plugins %q and %q", appID, existing.ID, manifest.ID)
			}
			catalog.byApplicationID[appID] = manifest
		}
		if botUserID := strings.TrimSpace(manifest.DiscordBotUserID); botUserID != "" {
			if existing, ok := catalog.byBotUserID[botUserID]; ok {
				return StaticCatalog{}, fmt.Errorf("duplicate Discord bot user ID %q for plugins %q and %q", botUserID, existing.ID, manifest.ID)
			}
			catalog.byBotUserID[botUserID] = manifest
		}
	}
	return catalog, nil
}

func (c StaticCatalog) FindByDiscordIdentity(applicationID string, botUserID string) (Manifest, bool) {
	if applicationID = strings.TrimSpace(applicationID); applicationID != "" {
		if manifest, ok := c.byApplicationID[applicationID]; ok {
			return manifest, true
		}
	}
	if botUserID = strings.TrimSpace(botUserID); botUserID != "" {
		if manifest, ok := c.byBotUserID[botUserID]; ok {
			return manifest, true
		}
	}
	return Manifest{}, false
}
