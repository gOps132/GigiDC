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
			command, args, ok := matchPrefixTrigger(trigger.Value, text)
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

func matchPrefixTrigger(trigger string, text string) (string, string, bool) {
	trigger = strings.TrimSpace(trigger)
	if trigger == "" {
		return "", "", false
	}
	if args, ok := matchCommand(text, trigger); ok {
		return buildCommand(trigger, args), args, true
	}
	bare := bareCommand(trigger)
	if bare == "" || bare == trigger {
		return "", "", false
	}
	if args, ok := matchCommand(text, bare); ok {
		return buildCommand(trigger, args), args, true
	}
	return "", "", false
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
