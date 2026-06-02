package plugins

type Manifest struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Version      string       `json:"version"`
	Source       string       `json:"source"`
	Capabilities []Capability `json:"capabilities"`
	Triggers     []Trigger    `json:"triggers"`
	Surfaces     []string     `json:"surfaces"`
	Permissions  []string     `json:"permissions"`
	ConfigSchema string       `json:"config_schema"`
	AuditEvents  []string     `json:"audit_events"`
	Attribution  []Resource   `json:"attribution"`
}

type Capability struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Trigger struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type Resource struct {
	Name   string `json:"name"`
	Use    string `json:"use"`
	Source string `json:"source"`
}

type Registry interface {
	EnabledForGuild(guildID string) ([]Manifest, error)
}
