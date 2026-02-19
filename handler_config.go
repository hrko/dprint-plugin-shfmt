package main

import (
	"fmt"
	"sort"

	"github.com/hrko/dprint-plugin-shfmt/dprint"
)

type configuration struct {
	IndentWidth      uint32 `json:"indentWidth"`
	UseTabs          bool   `json:"useTabs"`
	BinaryNextLine   bool   `json:"binaryNextLine"`
	SwitchCaseIndent bool   `json:"switchCaseIndent"`
	SpaceRedirects   bool   `json:"spaceRedirects"`
	FuncNextLine     bool   `json:"funcNextLine"`
	Minify           bool   `json:"minify"`
}

const (
	configKeyLocked           = "locked"
	configKeyIndentWidth      = "indentWidth"
	configKeyUseTabs          = "useTabs"
	configKeyBinaryNextLine   = "binaryNextLine"
	configKeySwitchCaseIndent = "switchCaseIndent"
	configKeySpaceRedirects   = "spaceRedirects"
	configKeyFuncNextLine     = "funcNextLine"
	configKeyMinify           = "minify"
)

type uint32ConfigFieldSpec struct {
	key                 string
	defaultValue        uint32
	allowGlobalOverride bool
	get                 func(configuration) uint32
	set                 func(*configuration, uint32)
}

type boolConfigFieldSpec struct {
	key                 string
	defaultValue        bool
	allowGlobalOverride bool
	get                 func(configuration) bool
	set                 func(*configuration, bool)
}

var (
	uint32ConfigFieldSpecs = []uint32ConfigFieldSpec{
		{
			key:                 configKeyIndentWidth,
			defaultValue:        2,
			allowGlobalOverride: true,
			get: func(config configuration) uint32 {
				return config.IndentWidth
			},
			set: func(config *configuration, value uint32) {
				config.IndentWidth = value
			},
		},
	}
	boolConfigFieldSpecs = []boolConfigFieldSpec{
		{
			key:                 configKeyUseTabs,
			defaultValue:        false,
			allowGlobalOverride: true,
			get: func(config configuration) bool {
				return config.UseTabs
			},
			set: func(config *configuration, value bool) {
				config.UseTabs = value
			},
		},
		{
			key:                 configKeyBinaryNextLine,
			defaultValue:        false,
			allowGlobalOverride: false,
			get: func(config configuration) bool {
				return config.BinaryNextLine
			},
			set: func(config *configuration, value bool) {
				config.BinaryNextLine = value
			},
		},
		{
			key:                 configKeySwitchCaseIndent,
			defaultValue:        false,
			allowGlobalOverride: false,
			get: func(config configuration) bool {
				return config.SwitchCaseIndent
			},
			set: func(config *configuration, value bool) {
				config.SwitchCaseIndent = value
			},
		},
		{
			key:                 configKeySpaceRedirects,
			defaultValue:        false,
			allowGlobalOverride: false,
			get: func(config configuration) bool {
				return config.SpaceRedirects
			},
			set: func(config *configuration, value bool) {
				config.SpaceRedirects = value
			},
		},
		{
			key:                 configKeyFuncNextLine,
			defaultValue:        false,
			allowGlobalOverride: false,
			get: func(config configuration) bool {
				return config.FuncNextLine
			},
			set: func(config *configuration, value bool) {
				config.FuncNextLine = value
			},
		},
		{
			key:                 configKeyMinify,
			defaultValue:        false,
			allowGlobalOverride: false,
			get: func(config configuration) bool {
				return config.Minify
			},
			set: func(config *configuration, value bool) {
				config.Minify = value
			},
		},
	}
	knownPropertySet = createKnownPropertySet()

	fileExtensions = []string{"sh", "bash", "zsh", "ksh", "bats"}
)

func (h *handler) ResolveConfig(config dprint.ConfigKeyMap, global dprint.GlobalConfiguration) dprint.ResolveConfigurationResult[configuration] {
	diagnostics := unknownPropertyDiagnostics(config)
	resolved := defaultConfiguration()

	applyConfigValues(&resolved, config, &diagnostics)
	applyGlobalOverrides(&resolved, global, &diagnostics)

	return dprint.ResolveConfigurationResult[configuration]{
		FileMatching: dprint.FileMatchingInfo{
			FileExtensions: append([]string(nil), fileExtensions...),
			FileNames:      []string{},
		},
		Diagnostics: diagnostics,
		Config:      resolved,
	}
}

func defaultConfiguration() configuration {
	var resolved configuration
	for _, field := range uint32ConfigFieldSpecs {
		field.set(&resolved, field.defaultValue)
	}
	for _, field := range boolConfigFieldSpecs {
		field.set(&resolved, field.defaultValue)
	}
	return resolved
}

func applyConfigValues(resolved *configuration, config map[string]any, diagnostics *[]dprint.ConfigurationDiagnostic) {
	for _, field := range uint32ConfigFieldSpecs {
		field.set(resolved, getUInt32(config, field.key, field.get(*resolved), diagnostics))
	}
	for _, field := range boolConfigFieldSpecs {
		field.set(resolved, getBool(config, field.key, field.get(*resolved), diagnostics))
	}
}

func applyGlobalOverrides(resolved *configuration, global map[string]any, diagnostics *[]dprint.ConfigurationDiagnostic) {
	for _, field := range uint32ConfigFieldSpecs {
		if !field.allowGlobalOverride {
			continue
		}
		field.set(resolved, getUInt32(global, field.key, field.get(*resolved), diagnostics))
	}
	for _, field := range boolConfigFieldSpecs {
		if !field.allowGlobalOverride {
			continue
		}
		field.set(resolved, getBool(global, field.key, field.get(*resolved), diagnostics))
	}
}

func unknownPropertyDiagnostics(config dprint.ConfigKeyMap) []dprint.ConfigurationDiagnostic {
	if len(config) == 0 {
		return nil
	}

	unknownKeys := make([]string, 0)
	for key := range config {
		if !isKnownProperty(key) {
			unknownKeys = append(unknownKeys, key)
		}
	}
	sort.Strings(unknownKeys)

	diagnostics := make([]dprint.ConfigurationDiagnostic, 0, len(unknownKeys))
	for _, key := range unknownKeys {
		diagnostics = append(diagnostics, dprint.ConfigurationDiagnostic{
			"propertyName": key,
			"message":      fmt.Sprintf("Unknown property '%s'.", key),
		})
	}
	return diagnostics
}

func createKnownPropertySet() map[string]struct{} {
	keys := make(map[string]struct{}, len(uint32ConfigFieldSpecs)+len(boolConfigFieldSpecs)+1)
	keys[configKeyLocked] = struct{}{}
	for _, field := range uint32ConfigFieldSpecs {
		keys[field.key] = struct{}{}
	}
	for _, field := range boolConfigFieldSpecs {
		keys[field.key] = struct{}{}
	}
	return keys
}

func isKnownProperty(key string) bool {
	_, ok := knownPropertySet[key]
	return ok
}

func getUInt32(config map[string]any, key string, fallback uint32, diagnostics *[]dprint.ConfigurationDiagnostic) uint32 {
	value, ok := config[key]
	if !ok {
		return fallback
	}
	if value == nil {
		return fallback
	}

	uintValue, ok := dprint.CoerceUInt32(value)
	if !ok {
		*diagnostics = append(*diagnostics, dprint.ConfigurationDiagnostic{
			"propertyName": key,
			"message":      fmt.Sprintf("Expected '%s' to be a non-negative integer, but got %T.", key, value),
		})
		return fallback
	}

	return uintValue
}

func getBool(config map[string]any, key string, fallback bool, diagnostics *[]dprint.ConfigurationDiagnostic) bool {
	value, ok := config[key]
	if !ok {
		return fallback
	}
	if value == nil {
		return fallback
	}

	boolValue, ok := dprint.CoerceBool(value)
	if !ok {
		*diagnostics = append(*diagnostics, dprint.ConfigurationDiagnostic{
			"propertyName": key,
			"message":      fmt.Sprintf("Expected '%s' to be a boolean, but got %T.", key, value),
		})
		return fallback
	}

	return boolValue
}
