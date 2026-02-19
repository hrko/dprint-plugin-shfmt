package dprint

import (
	"encoding/json"
	"testing"
)

type resolveConfigSpecTestConfig struct {
	IndentWidth uint32
	UseTabs     bool
	Minify      bool
}

var resolveConfigSpecTestSpec = ConfigResolverSpec[resolveConfigSpecTestConfig]{
	UInt32Fields: []UInt32ConfigFieldSpec[resolveConfigSpecTestConfig]{
		{
			Key:                 "indentWidth",
			DefaultValue:        2,
			AllowGlobalOverride: true,
			Get: func(config resolveConfigSpecTestConfig) uint32 {
				return config.IndentWidth
			},
			Set: func(config *resolveConfigSpecTestConfig, value uint32) {
				config.IndentWidth = value
			},
		},
	},
	BoolFields: []BoolConfigFieldSpec[resolveConfigSpecTestConfig]{
		{
			Key:                 "useTabs",
			DefaultValue:        false,
			AllowGlobalOverride: true,
			Get: func(config resolveConfigSpecTestConfig) bool {
				return config.UseTabs
			},
			Set: func(config *resolveConfigSpecTestConfig, value bool) {
				config.UseTabs = value
			},
		},
		{
			Key:                 "minify",
			DefaultValue:        false,
			AllowGlobalOverride: false,
			Get: func(config resolveConfigSpecTestConfig) bool {
				return config.Minify
			},
			Set: func(config *resolveConfigSpecTestConfig, value bool) {
				config.Minify = value
			},
		},
	},
	KnownKeys: []string{
		"locked",
		"indentWidth",
		"useTabs",
		"minify",
	},
}

func TestResolveConfigWithSpecDefaults(t *testing.T) {
	resolved, diagnostics := ResolveConfigWithSpec(
		ConfigKeyMap{},
		GlobalConfiguration{},
		resolveConfigSpecTestSpec,
	)

	if resolved.IndentWidth != 2 {
		t.Fatalf("expected indentWidth=2, got %d", resolved.IndentWidth)
	}
	if resolved.UseTabs {
		t.Fatal("expected useTabs=false")
	}
	if resolved.Minify {
		t.Fatal("expected minify=false")
	}
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %d", len(diagnostics))
	}
}

func TestResolveConfigWithSpecPrefersPluginConfigOverGlobal(t *testing.T) {
	resolved, diagnostics := ResolveConfigWithSpec(
		ConfigKeyMap{
			"indentWidth": float64(4),
			"useTabs":     []byte("false"),
			"minify":      []byte("true"),
			"unknown":     true,
		},
		GlobalConfiguration{
			"indentWidth": json.Number("8"),
			"useTabs":     []byte("1"),
		},
		resolveConfigSpecTestSpec,
	)

	if resolved.IndentWidth != 4 {
		t.Fatalf("expected indentWidth=4, got %d", resolved.IndentWidth)
	}
	if resolved.UseTabs {
		t.Fatal("expected useTabs=false")
	}
	if !resolved.Minify {
		t.Fatal("expected minify=true")
	}
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnostics))
	}
	if diagnostics[0]["propertyName"] != "unknown" {
		t.Fatalf("expected unknown property diagnostic, got %#v", diagnostics[0])
	}
}

func TestResolveConfigWithSpecAllowsKnownExtraKey(t *testing.T) {
	_, diagnostics := ResolveConfigWithSpec(
		ConfigKeyMap{
			"locked": true,
		},
		GlobalConfiguration{},
		resolveConfigSpecTestSpec,
	)

	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics for locked key, got %d", len(diagnostics))
	}
}

func TestResolveConfigWithSpecIgnoresNilValues(t *testing.T) {
	resolved, diagnostics := ResolveConfigWithSpec(
		ConfigKeyMap{
			"indentWidth": nil,
			"useTabs":     nil,
		},
		GlobalConfiguration{
			"indentWidth": nil,
			"useTabs":     nil,
		},
		resolveConfigSpecTestSpec,
	)

	if resolved.IndentWidth != 2 {
		t.Fatalf("expected fallback indentWidth=2, got %d", resolved.IndentWidth)
	}
	if resolved.UseTabs {
		t.Fatal("expected fallback useTabs=false")
	}
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %d", len(diagnostics))
	}
}
