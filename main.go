// Package main implements the dprint-plugin-shfmt Wasm entrypoint.
package main

import (
	"github.com/hrko/dprint-plugin-shfmt/dprint"
)

//go:generate go run github.com/hrko/dprint-plugin-shfmt/dprint/cmd/gen-main-boilerplate -runtime runtime -out main_generated.go

type configuration struct {
	EnsureFinalNewline bool `json:"ensureFinalNewline"`
}

type handler struct{}

func (h *handler) ResolveConfig(config dprint.ConfigKeyMap, _ dprint.GlobalConfiguration) dprint.ResolveConfigurationResult[configuration] {
	ensureFinalNewline := false
	if value, ok := config["ensureFinalNewline"]; ok {
		if boolValue, ok := value.(bool); ok {
			ensureFinalNewline = boolValue
		}
	}

	return dprint.ResolveConfigurationResult[configuration]{
		FileMatching: dprint.FileMatchingInfo{
			FileExtensions: []string{"sh", "bash", "zsh"},
			FileNames:      []string{},
		},
		Diagnostics: []dprint.ConfigurationDiagnostic{},
		Config: configuration{
			EnsureFinalNewline: ensureFinalNewline,
		},
	}
}

func (h *handler) PluginInfo() dprint.PluginInfo {
	return dprint.PluginInfo{
		Name:            "dprint-plugin-shfmt",
		Version:         "0.0.0-dev",
		ConfigKey:       "shfmt",
		HelpURL:         "https://github.com/dprint/dprint",
		ConfigSchemaURL: "",
	}
}

func (h *handler) LicenseText() string {
	return "MIT"
}

func (h *handler) CheckConfigUpdates(_ dprint.CheckConfigUpdatesMessage) ([]dprint.ConfigChange, error) {
	return []dprint.ConfigChange{}, nil
}

func (h *handler) Format(
	request dprint.SyncFormatRequest[configuration],
	_ dprint.HostFormatFunc,
) dprint.FormatResult {
	if request.Config.EnsureFinalNewline &&
		len(request.FileBytes) > 0 &&
		request.FileBytes[len(request.FileBytes)-1] != '\n' {
		formatted := make([]byte, len(request.FileBytes)+1)
		copy(formatted, request.FileBytes)
		formatted[len(formatted)-1] = '\n'
		return dprint.Change(formatted)
	}

	return dprint.NoChange()
}

var runtime = dprint.NewRuntime(&handler{})
