package main

import "github.com/hrko/dprint-plugin-shfmt/dprint"

//go:generate go run github.com/hrko/dprint-plugin-shfmt/dprint/cmd/gen-config-resolver -type configuration -out handler_config_generated.go -extra-known-keys locked
//go:generate go run github.com/hrko/dprint-plugin-shfmt/dprint/cmd/gen-json-schema -type configuration -out schema.json -schema-id https://raw.githubusercontent.com/hrko/dprint-plugin-shfmt/main/schema.json -include-locked -locked-description "Whether the configuration is not allowed to be overridden or extended."

type configuration struct {
	IndentWidth      uint32 `description:"Number of spaces per indentation level when not using tabs."                                        dprint:"default=2,global"     json:"indentWidth"`
	UseTabs          bool   `description:"Whether to use tabs for indentation."                                                               dprint:"default=false,global" json:"useTabs"`
	BinaryNextLine   bool   `description:"Whether binary operators should be placed at the start of the next line when line wrapping occurs." dprint:"default=false"        json:"binaryNextLine"`
	SwitchCaseIndent bool   `description:"Whether switch case bodies should be indented."                                                     dprint:"default=false"        json:"switchCaseIndent"`
	SpaceRedirects   bool   `description:"Whether to insert a space after redirection operators."                                             dprint:"default=false"        json:"spaceRedirects"`
	FuncNextLine     bool   `description:"Whether to place function opening braces on the next line."                                         dprint:"default=false"        json:"funcNextLine"`
	Minify           bool   `description:"Whether to minify shell scripts when printing."                                                     dprint:"default=false"        json:"minify"`
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
