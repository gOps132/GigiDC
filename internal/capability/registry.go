package capability

type Preset struct {
	Name         string
	Capabilities []Capability
}

func KnownCapabilities() []Capability {
	return []Capability{
		CapabilityManage,
		"plugin.install",
		"job.admin",
		"agent.analytics",
		"memory.read.guild",
		"memory.manage.guild",
		"relay.dispatch",
		"relay.receive",
		"llm.provider.write",
		"llm.provider.test",
		"llm.provider.select",
	}
}

func KnownPresets() []Preset {
	return []Preset{
		{
			Name: "gigi-admin",
			Capabilities: []Capability{
				CapabilityManage,
				"plugin.install",
				"job.admin",
				"agent.analytics",
				"memory.read.guild",
				"memory.manage.guild",
				"llm.provider.write",
				"llm.provider.test",
				"llm.provider.select",
			},
		},
		{Name: "plugin-manager", Capabilities: []Capability{"plugin.install"}},
		{Name: "memory-manager", Capabilities: []Capability{"memory.read.guild", "memory.manage.guild"}},
		{
			Name: "llm-manager",
			Capabilities: []Capability{
				"llm.provider.write",
				"llm.provider.test",
				"llm.provider.select",
			},
		},
		{Name: "memory-reader", Capabilities: []Capability{"memory.read.guild"}},
		{Name: "relay-user", Capabilities: []Capability{"relay.dispatch", "relay.receive"}},
	}
}

func PresetCapabilities(name string) ([]Capability, bool) {
	for _, preset := range KnownPresets() {
		if preset.Name == name {
			return append([]Capability(nil), preset.Capabilities...), true
		}
	}
	return nil, false
}
