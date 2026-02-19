package dprint

import (
	"fmt"
	"sort"
)

// UInt32ConfigFieldSpec describes how to resolve one uint32 configuration field.
type UInt32ConfigFieldSpec[T any] struct {
	Key                 string
	DefaultValue        uint32
	AllowGlobalOverride bool
	Get                 func(config T) uint32
	Set                 func(config *T, value uint32)
}

// BoolConfigFieldSpec describes how to resolve one bool configuration field.
type BoolConfigFieldSpec[T any] struct {
	Key                 string
	DefaultValue        bool
	AllowGlobalOverride bool
	Get                 func(config T) bool
	Set                 func(config *T, value bool)
}

// ConfigResolverSpec declares all fields used for configuration resolution.
type ConfigResolverSpec[T any] struct {
	UInt32Fields []UInt32ConfigFieldSpec[T]
	BoolFields   []BoolConfigFieldSpec[T]
	KnownKeys    []string
}

// ResolveConfigWithSpec resolves plugin and global settings based on field specs.
func ResolveConfigWithSpec[T any](
	config ConfigKeyMap,
	global GlobalConfiguration,
	spec ConfigResolverSpec[T],
) (T, []ConfigurationDiagnostic) {
	diagnostics := unknownPropertyDiagnosticsWithKnownKeys(config, knownConfigKeys(spec))
	resolved := defaultConfigurationFromSpec(spec)

	applyGlobalOverridesWithSpec(&resolved, global, spec, &diagnostics)
	applyConfigValuesWithSpec(&resolved, config, spec, &diagnostics)

	return resolved, diagnostics
}

func defaultConfigurationFromSpec[T any](spec ConfigResolverSpec[T]) T {
	var resolved T

	for _, field := range spec.UInt32Fields {
		field.Set(&resolved, field.DefaultValue)
	}
	for _, field := range spec.BoolFields {
		field.Set(&resolved, field.DefaultValue)
	}

	return resolved
}

func applyConfigValuesWithSpec[T any](
	resolved *T,
	config map[string]any,
	spec ConfigResolverSpec[T],
	diagnostics *[]ConfigurationDiagnostic,
) {
	for _, field := range spec.UInt32Fields {
		field.Set(
			resolved,
			getUInt32(config, field.Key, field.Get(*resolved), diagnostics),
		)
	}
	for _, field := range spec.BoolFields {
		field.Set(
			resolved,
			getBool(config, field.Key, field.Get(*resolved), diagnostics),
		)
	}
}

func applyGlobalOverridesWithSpec[T any](
	resolved *T,
	global map[string]any,
	spec ConfigResolverSpec[T],
	diagnostics *[]ConfigurationDiagnostic,
) {
	for _, field := range spec.UInt32Fields {
		if !field.AllowGlobalOverride {
			continue
		}

		field.Set(
			resolved,
			getUInt32(global, field.Key, field.Get(*resolved), diagnostics),
		)
	}

	for _, field := range spec.BoolFields {
		if !field.AllowGlobalOverride {
			continue
		}

		field.Set(
			resolved,
			getBool(global, field.Key, field.Get(*resolved), diagnostics),
		)
	}
}

func knownConfigKeys[T any](spec ConfigResolverSpec[T]) []string {
	if len(spec.KnownKeys) > 0 {
		return spec.KnownKeys
	}

	keys := make([]string, 0, len(spec.UInt32Fields)+len(spec.BoolFields))
	for _, field := range spec.UInt32Fields {
		keys = append(keys, field.Key)
	}
	for _, field := range spec.BoolFields {
		keys = append(keys, field.Key)
	}
	return keys
}

func unknownPropertyDiagnosticsWithKnownKeys(config map[string]any, knownKeys []string) []ConfigurationDiagnostic {
	if len(config) == 0 {
		return nil
	}

	knownKeySet := make(map[string]struct{}, len(knownKeys))
	for _, key := range knownKeys {
		knownKeySet[key] = struct{}{}
	}

	unknownKeys := make([]string, 0)
	for key := range config {
		if _, ok := knownKeySet[key]; ok {
			continue
		}

		unknownKeys = append(unknownKeys, key)
	}
	sort.Strings(unknownKeys)

	diagnostics := make([]ConfigurationDiagnostic, 0, len(unknownKeys))
	for _, key := range unknownKeys {
		diagnostics = append(diagnostics, ConfigurationDiagnostic{
			"propertyName": key,
			"message":      fmt.Sprintf("Unknown property '%s'.", key),
		})
	}

	return diagnostics
}

func getUInt32(
	config map[string]any,
	key string,
	fallback uint32,
	diagnostics *[]ConfigurationDiagnostic,
) uint32 {
	value, ok := config[key]
	if !ok {
		return fallback
	}
	if value == nil {
		return fallback
	}

	uintValue, ok := CoerceUInt32(value)
	if !ok {
		*diagnostics = append(*diagnostics, ConfigurationDiagnostic{
			"propertyName": key,
			"message":      fmt.Sprintf("Expected '%s' to be a non-negative integer, but got %T.", key, value),
		})
		return fallback
	}

	return uintValue
}

func getBool(
	config map[string]any,
	key string,
	fallback bool,
	diagnostics *[]ConfigurationDiagnostic,
) bool {
	value, ok := config[key]
	if !ok {
		return fallback
	}
	if value == nil {
		return fallback
	}

	boolValue, ok := CoerceBool(value)
	if !ok {
		*diagnostics = append(*diagnostics, ConfigurationDiagnostic{
			"propertyName": key,
			"message":      fmt.Sprintf("Expected '%s' to be a boolean, but got %T.", key, value),
		})
		return fallback
	}

	return boolValue
}
