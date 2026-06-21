package capability

import "testing"

func TestKnownCapabilitiesIncludeFixedPermissionChoices(t *testing.T) {
	caps := KnownCapabilities()
	for _, want := range []Capability{
		CapabilityManage,
		"plugin.install",
		"job.admin",
		"job.read",
		"job.schedule",
		"job.write",
		"web.search",
		"web.fetch",
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
	if !contains(adminCaps, "memory.read.guild") || !contains(adminCaps, "memory.manage.guild") || !contains(adminCaps, "agent.analytics") || !contains(adminCaps, "agent.reply_latency.manage") || !contains(adminCaps, "web.fetch") || !contains(adminCaps, "job.schedule") {
		t.Fatalf("gigi-admin caps = %+v, want memory, agent, web, and job capabilities", adminCaps)
	}

	memoryCaps, ok := PresetCapabilities("memory-manager")
	if !ok {
		t.Fatal("memory-manager preset missing")
	}
	if !contains(memoryCaps, "memory.read.guild") || !contains(memoryCaps, "memory.manage.guild") {
		t.Fatalf("memory-manager caps = %+v, want memory read/manage", memoryCaps)
	}

	webCaps, ok := PresetCapabilities("web-reader")
	if !ok {
		t.Fatal("web-reader preset missing")
	}
	if !contains(webCaps, "web.search") || !contains(webCaps, "web.fetch") {
		t.Fatalf("web-reader caps = %+v, want web search/fetch", webCaps)
	}

	jobCaps, ok := PresetCapabilities("job-operator")
	if !ok {
		t.Fatal("job-operator preset missing")
	}
	if !contains(jobCaps, "job.read") || !contains(jobCaps, "job.schedule") || !contains(jobCaps, "job.write") {
		t.Fatalf("job-operator caps = %+v, want job read/schedule/write", jobCaps)
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
