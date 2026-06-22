package provider

import (
	"strings"
	"testing"
)

func TestDefaultRegistryContainsExpectedProviderSpecs(t *testing.T) {
	registry := DefaultRegistry()

	for _, providerID := range []ProviderID{ProviderOpenAI, ProviderAnthropic, ProviderGemini, ProviderCustom} {
		spec, ok := registry.Spec(providerID)
		if !ok {
			t.Fatalf("DefaultRegistry missing provider %q", providerID)
		}
		if spec.ID != providerID {
			t.Fatalf("spec ID = %q, want %q", spec.ID, providerID)
		}
		if strings.TrimSpace(spec.DisplayName) == "" {
			t.Fatalf("provider %q has empty display name", providerID)
		}
		if len(spec.SupportedPurposes) == 0 {
			t.Fatalf("provider %q has no supported purposes", providerID)
		}
	}
}

func TestRegistryValidatesKnownProviders(t *testing.T) {
	registry := DefaultRegistry()

	for _, providerID := range []ProviderID{ProviderOpenAI, ProviderAnthropic, ProviderGemini, ProviderCustom} {
		if err := registry.ValidateProvider(providerID); err != nil {
			t.Fatalf("ValidateProvider(%q) returned error: %v", providerID, err)
		}
	}

	if err := registry.ValidateProvider("unknown"); err == nil {
		t.Fatal("ValidateProvider accepted unknown provider")
	}
}

func TestValidateProviderUsesDefaultRegistry(t *testing.T) {
	if err := ValidateProvider(ProviderOpenAI); err != nil {
		t.Fatalf("ValidateProvider returned error for OpenAI: %v", err)
	}
	if err := ValidateProvider("unknown"); err == nil {
		t.Fatal("ValidateProvider accepted unknown provider")
	}
}

func TestValidatePurposeAcceptsOnlyKnownPurposes(t *testing.T) {
	for _, purpose := range []Purpose{PurposeChat, PurposeReasoning, PurposeEmbedding, PurposeRouting} {
		if err := ValidatePurpose(purpose); err != nil {
			t.Fatalf("ValidatePurpose(%q) returned error: %v", purpose, err)
		}
	}

	if err := ValidatePurpose("summarizing"); err == nil {
		t.Fatal("ValidatePurpose accepted unknown purpose")
	}
}

func TestValidateOwnerTypeAcceptsOnlyKnownOwnerTypes(t *testing.T) {
	for _, ownerType := range []OwnerType{OwnerGuild, OwnerUser, OwnerTenant} {
		if err := ValidateOwnerType(ownerType); err != nil {
			t.Fatalf("ValidateOwnerType(%q) returned error: %v", ownerType, err)
		}
	}

	if err := ValidateOwnerType("workspace"); err == nil {
		t.Fatal("ValidateOwnerType accepted unknown owner type")
	}
}

func TestSupportsPurposeUsesProviderSpec(t *testing.T) {
	registry := DefaultRegistry()

	if !registry.SupportsPurpose(ProviderOpenAI, PurposeEmbedding) {
		t.Fatal("OpenAI should support embeddings")
	}
	if !registry.SupportsPurpose(ProviderAnthropic, PurposeReasoning) {
		t.Fatal("Anthropic should support reasoning")
	}
	if registry.SupportsPurpose(ProviderAnthropic, PurposeEmbedding) {
		t.Fatal("Anthropic should not support embeddings in default registry")
	}
	if registry.SupportsPurpose("unknown", PurposeChat) {
		t.Fatal("unknown provider should not support any purpose")
	}
	if registry.SupportsPurpose(ProviderOpenAI, "unknown") {
		t.Fatal("known provider should not support unknown purpose")
	}
}

func TestSupportsPurposeUsesDefaultRegistry(t *testing.T) {
	if !SupportsPurpose(ProviderOpenAI, PurposeEmbedding) {
		t.Fatal("OpenAI should support embeddings")
	}
	if SupportsPurpose(ProviderAnthropic, PurposeEmbedding) {
		t.Fatal("Anthropic should not support embeddings in default registry")
	}
}

func TestDefaultModelSelectsProviderPurposeDefaults(t *testing.T) {
	tests := []struct {
		name       string
		providerID ProviderID
		purpose    Purpose
		want       string
	}{
		{name: "openai chat", providerID: ProviderOpenAI, purpose: PurposeChat, want: "gpt-5.4-mini"},
		{name: "openai reasoning", providerID: ProviderOpenAI, purpose: PurposeReasoning, want: "gpt-5.5"},
		{name: "openai embedding", providerID: ProviderOpenAI, purpose: PurposeEmbedding, want: "text-embedding-3-small"},
		{name: "openai routing", providerID: ProviderOpenAI, purpose: PurposeRouting, want: "gpt-5.4-mini"},
		{name: "anthropic chat", providerID: ProviderAnthropic, purpose: PurposeChat, want: "claude-sonnet-4-6"},
		{name: "anthropic reasoning", providerID: ProviderAnthropic, purpose: PurposeReasoning, want: "claude-sonnet-4-6"},
		{name: "anthropic routing", providerID: ProviderAnthropic, purpose: PurposeRouting, want: "claude-haiku-4-5-20251001"},
		{name: "gemini chat", providerID: ProviderGemini, purpose: PurposeChat, want: "gemini-3.5-flash"},
		{name: "gemini reasoning", providerID: ProviderGemini, purpose: PurposeReasoning, want: "gemini-3.1-pro-preview"},
		{name: "gemini embedding", providerID: ProviderGemini, purpose: PurposeEmbedding, want: "gemini-embedding-2"},
		{name: "gemini routing", providerID: ProviderGemini, purpose: PurposeRouting, want: "gemini-3.5-flash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := DefaultModel(tt.providerID, tt.purpose)
			if !ok {
				t.Fatalf("DefaultModel(%q, %q) ok = false, want true", tt.providerID, tt.purpose)
			}
			if got != tt.want {
				t.Fatalf("DefaultModel(%q, %q) = %q, want %q", tt.providerID, tt.purpose, got, tt.want)
			}
		})
	}
}

func TestDefaultModelRejectsUnsupportedProviderPurposePairs(t *testing.T) {
	tests := []struct {
		name       string
		providerID ProviderID
		purpose    Purpose
	}{
		{name: "anthropic embedding", providerID: ProviderAnthropic, purpose: PurposeEmbedding},
		{name: "custom chat", providerID: ProviderCustom, purpose: PurposeChat},
		{name: "unknown provider", providerID: ProviderID("unknown"), purpose: PurposeChat},
		{name: "unknown purpose", providerID: ProviderOpenAI, purpose: Purpose("summarizing")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, ok := DefaultModel(tt.providerID, tt.purpose); ok {
				t.Fatalf("DefaultModel(%q, %q) = %q, true; want false", tt.providerID, tt.purpose, got)
			}
		})
	}
}

func TestValidateModelIDTrimsAndAllowsFlexibleModelNames(t *testing.T) {
	modelID, err := ValidateModelID("  provider/custom-model:v1.2  ")
	if err != nil {
		t.Fatalf("ValidateModelID returned error: %v", err)
	}
	if modelID != "provider/custom-model:v1.2" {
		t.Fatalf("modelID = %q, want trimmed flexible value", modelID)
	}
}

func TestValidateModelIDRejectsUnsafeValues(t *testing.T) {
	longModelID := strings.Repeat("a", 161)
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "whitespace only", input: " \t\n "},
		{name: "too long", input: longModelID},
		{name: "control character", input: "gpt-4o\nmini"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ValidateModelID(tt.input); err == nil {
				t.Fatalf("ValidateModelID(%q) returned nil error", tt.input)
			}
		})
	}
}
