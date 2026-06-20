package audit

import (
	"fmt"
	"strings"
)

type Status string

const (
	StatusAllowed   Status = "allowed"
	StatusDenied    Status = "denied"
	StatusFailed    Status = "failed"
	StatusSucceeded Status = "succeeded"
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

const redactedMetadataValue = "[REDACTED]"

func SanitizeMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return nil
	}
	sanitized := make(map[string]string, len(metadata))
	for key, value := range metadata {
		if IsSensitiveMetadata(key) {
			continue
		}
		if IsSensitiveMetadata(value) {
			sanitized[key] = redactedMetadataValue
			continue
		}
		sanitized[key] = value
	}
	return sanitized
}

func (e Event) Validate() error {
	if strings.TrimSpace(e.Kind) == "" {
		return fmt.Errorf("audit kind is required")
	}
	if strings.TrimSpace(e.ActorID) == "" {
		return fmt.Errorf("audit actor ID is required")
	}
	if !validStatus(e.Status) {
		return fmt.Errorf("audit status is required")
	}
	for key, value := range e.Metadata {
		if IsSensitiveMetadata(key) || IsSensitiveMetadata(value) {
			return fmt.Errorf("sensitive metadata is not allowed")
		}
	}
	return nil
}

func validStatus(status Status) bool {
	switch status {
	case StatusAllowed, StatusDenied, StatusFailed, StatusSucceeded:
		return true
	default:
		return false
	}
}

func IsSensitiveMetadata(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}
	if hasSecretKeyPrefix(value) {
		return true
	}
	for _, marker := range []string{
		"api_key",
		"apikey",
		"authorization",
		"bearer ",
		"client_secret",
		"provider_secret",
		"private_key",
		"refresh_token",
		"secret",
		"token",
		"x-api-key",
	} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func looksSensitive(value string) bool {
	return IsSensitiveMetadata(value)
}

func hasSecretKeyPrefix(value string) bool {
	if strings.HasPrefix(value, "sk-") {
		return true
	}
	for _, marker := range []string{" sk-", "\tsk-", "\nsk-", "\"sk-", "'sk-", "=sk-", ":sk-"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}
