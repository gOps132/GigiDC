package plugins

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gOps132/GigiDC/internal/capability"
)

type CommandPlan struct {
	Manifest             Manifest
	Trigger              Trigger
	Command              string
	Arguments            string
	RequiredCapabilities []capability.Capability
}

func PlanCommand(manifests []Manifest, surface string, text string) (CommandPlan, bool) {
	surface = strings.TrimSpace(surface)
	text = strings.TrimSpace(text)
	if surface == "" || text == "" {
		return CommandPlan{}, false
	}
	for _, manifest := range manifests {
		if !supportsSurface(manifest, surface) {
			continue
		}
		for _, trigger := range manifest.Triggers {
			if strings.TrimSpace(trigger.Kind) != "prefix" {
				continue
			}
			command, args, ok := matchPrefixTrigger(trigger, text)
			if !ok {
				continue
			}
			return CommandPlan{
				Manifest:             manifest,
				Trigger:              trigger,
				Command:              command,
				Arguments:            args,
				RequiredCapabilities: requiredCapabilities(manifest),
			}, true
		}
	}
	return CommandPlan{}, false
}

func supportsSurface(manifest Manifest, surface string) bool {
	for _, candidate := range manifest.Surfaces {
		if strings.TrimSpace(candidate) == surface {
			return true
		}
	}
	return false
}

func matchPrefixTrigger(trigger Trigger, text string) (string, string, bool) {
	value := strings.TrimSpace(trigger.Value)
	if value == "" {
		return "", "", false
	}
	if args, ok := matchCommand(text, value); ok {
		return buildCommand(value, args), args, true
	}
	for _, alias := range triggerAliases(trigger) {
		if args, ok := matchCommand(text, alias); ok {
			return buildCommand(value, args), args, true
		}
	}
	return "", "", false
}

func triggerAliases(trigger Trigger) []string {
	value := strings.TrimSpace(trigger.Value)
	aliases := make([]string, 0, len(trigger.Aliases)+1)
	bare := bareCommand(value)
	if bare != "" && !strings.EqualFold(bare, value) {
		aliases = append(aliases, bare)
	}
	for _, alias := range trigger.Aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" || strings.EqualFold(alias, value) {
			continue
		}
		aliases = append(aliases, alias)
	}
	return aliases
}

func matchCommand(text string, command string) (string, bool) {
	text = strings.TrimSpace(text)
	command = strings.TrimSpace(command)
	if len(text) < len(command) {
		return "", false
	}
	if !strings.EqualFold(text[:len(command)], command) {
		return "", false
	}
	if len(text) == len(command) {
		return "", true
	}
	next, _ := utf8.DecodeRuneInString(text[len(command):])
	if !unicode.IsSpace(next) {
		return "", false
	}
	return strings.TrimSpace(text[len(command):]), true
}

func bareCommand(command string) string {
	return strings.TrimLeftFunc(strings.TrimSpace(command), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func buildCommand(trigger string, args string) string {
	trigger = strings.TrimSpace(trigger)
	args = strings.TrimSpace(args)
	if args == "" {
		return trigger
	}
	return trigger + " " + args
}

func requiredCapabilities(manifest Manifest) []capability.Capability {
	capabilities := make([]capability.Capability, 0, len(manifest.Permissions))
	for _, permission := range manifest.Permissions {
		capability, err := capability.Normalize(permission)
		if err != nil {
			continue
		}
		capabilities = append(capabilities, capability)
	}
	return capabilities
}
