package capability

import "testing"

func TestKnownCapabilitiesIncludeFixedPermissionChoices(t *testing.T) {
	caps := KnownCapabilities()
	for _, want := range []Capability{
		CapabilityManage,
		"plugin.install",
		"job.admin",
		"memory.read.guild",
		"relay.dispatch",
		"relay.receive",
		"llm.provider.write",
		"llm.provider.test",
		"llm.provider.select",
	} {
		if !contains(caps, want) {
			t.Fatalf("KnownCapabilities missing %q in %+v", want, caps)
		}
	}
	if contains(caps, "plugin.run.<id>") {
		t.Fatalf("KnownCapabilities includes dynamic plugin.run placeholder: %+v", caps)
	}
}

func TestPresetCapabilities(t *testing.T) {
	caps, ok := PresetCapabilities("relay-user")
	if !ok {
		t.Fatal("relay-user preset missing")
	}
	if !contains(caps, "relay.dispatch") || !contains(caps, "relay.receive") {
		t.Fatalf("relay-user caps = %+v, want relay dispatch and receive", caps)
	}

	if _, ok := PresetCapabilities("missing"); ok {
		t.Fatal("missing preset returned ok")
	}
}
