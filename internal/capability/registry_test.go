package capability

import "testing"

func TestKnownCapabilitiesIncludeFixedPermissionChoices(t *testing.T) {
	caps := KnownCapabilities()
	for _, want := range []Capability{
		CapabilityManage,
		"plugin.install",
		"job.admin",
		"agent.analytics",
		"agent.reply_latency.manage",
		"memory.read.guild",
		"memory.manage.guild",
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
	adminCaps, ok := PresetCapabilities("gigi-admin")
	if !ok {
		t.Fatal("gigi-admin preset missing")
	}
	if !contains(adminCaps, "memory.read.guild") || !contains(adminCaps, "memory.manage.guild") || !contains(adminCaps, "agent.analytics") || !contains(adminCaps, "agent.reply_latency.manage") {
		t.Fatalf("gigi-admin caps = %+v, want memory read/manage and agent capabilities", adminCaps)
	}

	memoryCaps, ok := PresetCapabilities("memory-manager")
	if !ok {
		t.Fatal("memory-manager preset missing")
	}
	if !contains(memoryCaps, "memory.read.guild") || !contains(memoryCaps, "memory.manage.guild") {
		t.Fatalf("memory-manager caps = %+v, want memory read/manage", memoryCaps)
	}

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
