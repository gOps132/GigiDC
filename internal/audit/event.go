package audit

import (
	"fmt"
	"strings"
)

type Status string

const (
	StatusAllowed Status = "allowed"
	StatusDenied  Status = "denied"
	StatusFailed  Status = "failed"
)

type Event struct {
	Kind      string
	GuildID   string
	ActorID   string
	Status    Status
	Reason    string
	Metadata  map[string]string
	RequestID string
}

func (e Event) Validate() error {
	if strings.TrimSpace(e.Kind) == "" {
		return fmt.Errorf("audit kind is required")
	}
	if strings.TrimSpace(e.ActorID) == "" {
		return fmt.Errorf("audit actor ID is required")
	}
	if e.Status == "" {
		return fmt.Errorf("audit status is required")
	}
	for key, value := range e.Metadata {
		if looksSensitive(key) || looksSensitive(value) {
			return fmt.Errorf("sensitive metadata is not allowed")
		}
	}
	return nil
}

func looksSensitive(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}
	for _, marker := range []string{
		"api_key",
		"apikey",
		"authorization",
		"bearer ",
		"provider_secret",
		"secret",
		"sk-",
		"token",
		"x-api-key",
	} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}
