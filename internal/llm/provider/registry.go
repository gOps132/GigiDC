package provider

import (
	"errors"
	"strings"
	"unicode"
)

type ProviderID string

const (
	ProviderOpenAI    ProviderID = "openai"
	ProviderAnthropic ProviderID = "anthropic"
	ProviderGemini    ProviderID = "gemini"
	ProviderCustom    ProviderID = "custom"
)

type Purpose string

const (
	PurposeChat      Purpose = "chat"
	PurposeReasoning Purpose = "reasoning"
	PurposeEmbedding Purpose = "embedding"
	PurposeRouting   Purpose = "routing"
)

type OwnerType string

const (
	OwnerGuild  OwnerType = "guild"
	OwnerUser   OwnerType = "user"
	OwnerTenant OwnerType = "tenant"
)

type ProviderSpec struct {
	ID                ProviderID
	DisplayName       string
	SupportedPurposes []Purpose
}

type Registry struct {
	specs map[ProviderID]ProviderSpec
}

func DefaultRegistry() Registry {
	return Registry{
		specs: map[ProviderID]ProviderSpec{
			ProviderOpenAI: {
				ID:          ProviderOpenAI,
				DisplayName: "OpenAI",
				SupportedPurposes: []Purpose{
					PurposeChat,
					PurposeReasoning,
					PurposeEmbedding,
					PurposeRouting,
				},
			},
			ProviderAnthropic: {
				ID:          ProviderAnthropic,
				DisplayName: "Anthropic",
				SupportedPurposes: []Purpose{
					PurposeChat,
					PurposeReasoning,
					PurposeRouting,
				},
			},
			ProviderGemini: {
				ID:          ProviderGemini,
				DisplayName: "Gemini",
				SupportedPurposes: []Purpose{
					PurposeChat,
					PurposeReasoning,
					PurposeEmbedding,
					PurposeRouting,
				},
			},
			ProviderCustom: {
				ID:          ProviderCustom,
				DisplayName: "Custom",
				SupportedPurposes: []Purpose{
					PurposeChat,
					PurposeReasoning,
					PurposeEmbedding,
					PurposeRouting,
				},
			},
		},
	}
}

func (r Registry) Spec(id ProviderID) (ProviderSpec, bool) {
	spec, ok := r.specs[id]
	if !ok {
		return ProviderSpec{}, false
	}
	spec.SupportedPurposes = append([]Purpose(nil), spec.SupportedPurposes...)
	return spec, true
}

func (r Registry) ValidateProvider(id ProviderID) error {
	if _, ok := r.specs[id]; ok {
		return nil
	}
	return errors.New("unknown provider")
}

func ValidateProvider(id ProviderID) error {
	return DefaultRegistry().ValidateProvider(id)
}

func ValidatePurpose(purpose Purpose) error {
	switch purpose {
	case PurposeChat, PurposeReasoning, PurposeEmbedding, PurposeRouting:
		return nil
	default:
		return errors.New("unknown purpose")
	}
}

func ValidateOwnerType(ownerType OwnerType) error {
	switch ownerType {
	case OwnerGuild, OwnerUser, OwnerTenant:
		return nil
	default:
		return errors.New("unknown owner type")
	}
}

func (r Registry) SupportsPurpose(id ProviderID, purpose Purpose) bool {
	spec, ok := r.specs[id]
	if !ok {
		return false
	}
	if ValidatePurpose(purpose) != nil {
		return false
	}
	for _, supportedPurpose := range spec.SupportedPurposes {
		if supportedPurpose == purpose {
			return true
		}
	}
	return false
}

func SupportsPurpose(id ProviderID, purpose Purpose) bool {
	return DefaultRegistry().SupportsPurpose(id, purpose)
}

func ValidateModelID(modelID string) (string, error) {
	trimmed := strings.TrimSpace(modelID)
	if trimmed == "" {
		return "", errors.New("model id is empty")
	}
	if len(trimmed) > 160 {
		return "", errors.New("model id is too long")
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			return "", errors.New("model id contains control characters")
		}
	}
	return trimmed, nil
}
