package main

import "github.com/hrko/dprint-plugin-shfmt/dprint"

//go:generate go run github.com/hrko/dprint-plugin-shfmt/dprint/cmd/gen-config-resolver -type configuration -out handler_config_generated.go -extra-known-keys locked

type configuration struct {
	IndentWidth      uint32 `dprint:"default=2,global"     json:"indentWidth"`
	UseTabs          bool   `dprint:"default=false,global" json:"useTabs"`
	BinaryNextLine   bool   `dprint:"default=false"        json:"binaryNextLine"`
	SwitchCaseIndent bool   `dprint:"default=false"        json:"switchCaseIndent"`
	SpaceRedirects   bool   `dprint:"default=false"        json:"spaceRedirects"`
	FuncNextLine     bool   `dprint:"default=false"        json:"funcNextLine"`
	Minify           bool   `dprint:"default=false"        json:"minify"`
}

var fileExtensions = []string{"sh", "bash", "zsh", "ksh", "bats"}

func (h *handler) ResolveConfig(
	config dprint.ConfigKeyMap,
	global dprint.GlobalConfiguration,
) dprint.ResolveConfigurationResult[configuration] {
	resolved, diagnostics := dprint.ResolveConfigWithSpec(
		config,
		global,
		generatedConfigurationResolverSpec,
	)

	return dprint.ResolveConfigurationResult[configuration]{
		FileMatching: dprint.FileMatchingInfo{
			FileExtensions: append([]string(nil), fileExtensions...),
			FileNames:      []string{},
		},
		Diagnostics: diagnostics,
		Config:      resolved,
	}
}
