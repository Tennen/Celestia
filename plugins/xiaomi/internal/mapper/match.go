package mapper

import (
	"strings"

	"github.com/chentianyu/celestia/plugins/xiaomi/internal/spec"
)

type matchProperty struct {
	names        []string
	serviceHints []string
	format       string
	boolOnly     bool
	excludeKinds []string
}

type matchAction struct {
	serviceHints []string
	requireInput bool
	stringInput  bool
}

func firstWritableProperty(services []serviceView, match matchProperty) *PropertyRef {
	return firstProperty(services, match, func(prop spec.Property) bool { return prop.Writable() })
}

func firstReadableProperty(services []serviceView, match matchProperty) *PropertyRef {
	return firstProperty(services, match, func(prop spec.Property) bool { return prop.Readable() || prop.Notifiable() })
}

func firstProperty(services []serviceView, match matchProperty, access func(spec.Property) bool) *PropertyRef {
	for _, service := range services {
		for _, prop := range service.properties {
			if !access(prop.Property) {
				continue
			}
			if matchPropertyRef(prop, match) {
				copy := prop
				return &copy
			}
		}
	}
	return nil
}

func firstAction(services []serviceView, match matchAction) *ActionRef {
	for _, service := range services {
		for _, action := range service.actions {
			if matchActionRef(action, match) {
				copy := action
				return &copy
			}
		}
	}
	return nil
}

func matchPropertyRef(prop PropertyRef, match matchProperty) bool {
	if match.boolOnly && prop.Property.Format != "bool" {
		return false
	}
	if match.format != "" && prop.Property.Format != match.format {
		return false
	}
	for _, format := range match.excludeKinds {
		if prop.Property.Format == format {
			return false
		}
	}
	name := spec.PropertyName(prop.Property)
	if len(match.names) > 0 && !containsNormalized(name, match.names) {
		return false
	}
	if len(match.serviceHints) > 0 && !containsAny(prop.ServiceName, strings.ToLower(prop.Property.Description), match.serviceHints...) {
		return false
	}
	return true
}

func matchActionRef(action ActionRef, match matchAction) bool {
	if match.requireInput && len(action.Inputs) == 0 {
		return false
	}
	if match.stringInput {
		hasString := false
		for _, input := range action.Inputs {
			if input.Format == "string" {
				hasString = true
				break
			}
		}
		if !hasString {
			return false
		}
	}
	if len(match.serviceHints) == 0 {
		return true
	}
	actionName := spec.ActionName(action.Action)
	return containsAny(action.ServiceName, actionName, match.serviceHints...)
}

func containsAny(haystackA, haystackB string, needles ...string) bool {
	return containsAnyImpl([]string{haystackA, haystackB}, needles)
}

func containsAnyImpl(haystacks []string, needles []string) bool {
	for _, haystack := range haystacks {
		haystack = strings.ToLower(haystack)
		for _, needle := range needles {
			needle = strings.ToLower(needle)
			if strings.Contains(haystack, needle) {
				return true
			}
		}
	}
	return false
}

func containsNormalized(value string, names []string) bool {
	value = strings.ToLower(value)
	for _, name := range names {
		if value == strings.ToLower(name) {
			return true
		}
	}
	return false
}
