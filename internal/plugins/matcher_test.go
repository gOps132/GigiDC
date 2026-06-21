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

func TestPlanCommandMatchesExplicitAliasForPrefixedTrigger(t *testing.T) {
	manifest := musicManifest()
	manifest.Triggers = []Trigger{{Kind: "prefix", Value: "m!play", Aliases: []string{"play"}}}

	plan, ok := PlanCommand([]Manifest{manifest}, "guild_text", "play never gonna give you up")
	if !ok {
		t.Fatal("expected explicit trigger alias match")
	}
	if plan.Command != "m!play never gonna give you up" || plan.Trigger.Value != "m!play" {
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
	if plan.Action.Safety != SafetyClassPublic {
		t.Fatalf("action safety = %q, want public", plan.Action.Safety)
	}
}

func TestPlanCommandUsesActionLevelContract(t *testing.T) {
	manifest := musicManifest()
	manifest.Triggers = nil
	manifest.Permissions = []string{"plugin.install"}
	manifest.Dispatch = DispatchModeDryRun
	manifest.Actions = []Action{
		{
			ID:          "play",
			Trigger:     Trigger{Kind: "prefix", Value: "!play", Aliases: []string{"play"}},
			Surfaces:    []string{"guild_text"},
			Permissions: nil,
			Safety:      SafetyClassPublic,
			Dispatch:    DispatchModeSendMessage,
			Adapter:     DispatchAdapterPrefixCommand,
		},
		{
			ID:          "admin-skip",
			Trigger:     Trigger{Kind: "prefix", Value: "!skip"},
			Surfaces:    []string{"guild_text"},
			Permissions: []string{"plugin.install"},
			Safety:      SafetyClassRestricted,
		},
	}

	plan, ok := PlanCommand([]Manifest{manifest}, "guild_text", "play never gonna give you up")
	if !ok {
		t.Fatal("expected action trigger match")
	}
	if plan.Action.ID != "play" || plan.Trigger.Value != "!play" || plan.Command != "!play never gonna give you up" {
		t.Fatalf("plan = %+v, want play action plan", plan)
	}
	if len(plan.RequiredCapabilities) != 0 || !plan.PublicAction() || plan.DispatchMode() != DispatchModeSendMessage {
		t.Fatalf("plan = %+v, want public send-message action", plan)
	}
}

func TestPlanCommandFromTriggerBuildsManifestGroundedPlan(t *testing.T) {
	plan, ok := PlanCommandFromTrigger([]Manifest{musicManifest()}, "guild_text", "jockie-music", "!play", "never gonna give you up")
	if !ok {
		t.Fatal("expected manifest-grounded trigger plan")
	}
	if plan.Command != "!play never gonna give you up" || plan.Arguments != "never gonna give you up" || plan.Trigger.Value != "!play" {
		t.Fatalf("plan = %+v, want grounded command", plan)
	}
	if len(plan.RequiredCapabilities) != 1 || plan.RequiredCapabilities[0] != capability.Capability("plugin.install") {
		t.Fatalf("required capabilities = %+v, want plugin.install", plan.RequiredCapabilities)
	}
}

func TestPlanCommandFromTriggerUsesActionLevelContract(t *testing.T) {
	manifest := musicManifest()
	manifest.Triggers = nil
	manifest.Actions = []Action{{
		ID:          "skip",
		Trigger:     Trigger{Kind: "prefix", Value: "!skip"},
		Surfaces:    []string{"guild_text"},
		Permissions: []string{"plugin.install"},
		Safety:      SafetyClassRestricted,
	}}

	plan, ok := PlanCommandFromTrigger([]Manifest{manifest}, "guild_text", "jockie-music", "!skip", "two tracks")
	if !ok {
		t.Fatal("expected action trigger plan")
	}
	if plan.Action.ID != "skip" || plan.Command != "!skip two tracks" || plan.Arguments != "two tracks" {
		t.Fatalf("plan = %+v, want action-grounded command", plan)
	}
	if len(plan.RequiredCapabilities) != 1 || plan.RequiredCapabilities[0] != capability.Capability("plugin.install") {
		t.Fatalf("required capabilities = %+v, want plugin.install", plan.RequiredCapabilities)
	}
}

func TestPlanCommandFromTriggerRejectsUnknownPluginOrTrigger(t *testing.T) {
	if _, ok := PlanCommandFromTrigger([]Manifest{musicManifest()}, "guild_text", "missing", "!play", "args"); ok {
		t.Fatal("must not accept unknown plugin")
	}
	if _, ok := PlanCommandFromTrigger([]Manifest{musicManifest()}, "guild_text", "jockie-music", "!missing", "args"); ok {
		t.Fatal("must not accept unknown trigger")
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
