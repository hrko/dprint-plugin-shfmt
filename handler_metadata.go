package main

import (
	_ "embed"
	"fmt"

	"github.com/hrko/dprint-plugin-shfmt/dprint"
)

const (
	pluginName      = "dprint-plugin-shfmt"
	pluginConfigKey = "shfmt"
	pluginHelpURL   = "https://github.com/hrko/dprint-plugin-shfmt"
	pluginUpdateURL = "https://plugins.dprint.dev/hrko/shfmt/latest.json"

	defaultVersion    = "0.0.0-dev"
	defaultReleaseTag = "v0.0.0-dev"
)

//go:generate sh -c "go-licenses report . --template licenses.tpl > licenses.generated.txt"
//go:embed licenses.generated.txt
var embeddedLicenseText string

func (h *handler) PluginInfo() dprint.PluginInfo {
	updateURL := pluginUpdateURL

	return dprint.PluginInfo{
		Name:            pluginName,
		Version:         orDefault(Version, defaultVersion),
		ConfigKey:       pluginConfigKey,
		HelpURL:         pluginHelpURL,
		ConfigSchemaURL: configSchemaURLForTag(orDefault(ReleaseTag, defaultReleaseTag)),
		UpdateURL:       &updateURL,
	}
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (h *handler) LicenseText() string {
	return embeddedLicenseText
}

func (h *handler) CheckConfigUpdates(_ dprint.CheckConfigUpdatesMessage) ([]dprint.ConfigChange, error) {
	return []dprint.ConfigChange{}, nil
}

func configSchemaURLForTag(tag string) string {
	return fmt.Sprintf("https://plugins.dprint.dev/hrko/shfmt/%s/schema.json", tag)
}
