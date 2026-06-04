package plugins

import (
	"testing"

	"github.com/gOps132/GigiDC/internal/capability"
)

func TestPlanCommandMatchesPrefixTrigger(t *testing.T) {
	manifest := musicManifest()

	plan, ok := PlanCommand([]Manifest{manifest}, "guild_text", "!play never gonna give you up")
	if !ok {
		t.Fatal("expected prefix trigger match")
	}
	if plan.Command != "!play never gonna give you up" || plan.Arguments != "never gonna give you up" {
		t.Fatalf("plan command/args = %q/%q", plan.Command, plan.Arguments)
	}
	if plan.Manifest.ID != "jockie-music" || plan.RequiredCapabilities[0] != capability.Capability("plugin.install") {
		t.Fatalf("plan = %+v, want jockie manifest with plugin.install", plan)
	}
}

func TestPlanCommandMatchesBareAliasForPrefixTrigger(t *testing.T) {
	plan, ok := PlanCommand([]Manifest{musicManifest()}, "guild_text", "play never gonna give you up")
	if !ok {
		t.Fatal("expected bare trigger alias match")
	}
	if plan.Command != "!play never gonna give you up" || plan.Trigger.Value != "!play" {
		t.Fatalf("plan = %+v, want normalized external app command", plan)
	}
}

func TestPlanCommandRequiresTriggerBoundary(t *testing.T) {
	if _, ok := PlanCommand([]Manifest{musicManifest()}, "guild_text", "playlist never gonna give you up"); ok {
		t.Fatal("must not match trigger inside a longer word")
	}
}

func TestPlanCommandSkipsSurfaceMismatch(t *testing.T) {
	if _, ok := PlanCommand([]Manifest{musicManifest()}, "dm", "play never gonna give you up"); ok {
		t.Fatal("must not match unsupported surface")
	}
}

func TestPlanCommandTreatsEmptyPermissionsAsPublic(t *testing.T) {
	manifest := musicManifest()
	manifest.Permissions = nil

	plan, ok := PlanCommand([]Manifest{manifest}, "guild_text", "play never gonna give you up")
	if !ok {
		t.Fatal("expected trigger match")
	}
	if len(plan.RequiredCapabilities) != 0 {
		t.Fatalf("required capabilities = %+v, want public action with no required capabilities", plan.RequiredCapabilities)
	}
}

func musicManifest() Manifest {
	manifest := validManifest()
	manifest.ID = "jockie-music"
	manifest.Name = "Jockie Music"
	manifest.Version = "1.0.0"
	manifest.Triggers = []Trigger{{Kind: "prefix", Value: "!play"}}
	manifest.Surfaces = []string{"guild_text"}
	manifest.Permissions = []string{"plugin.install"}
	return manifest
}
