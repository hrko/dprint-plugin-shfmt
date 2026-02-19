// Package main implements the dprint-plugin-shfmt Wasm entrypoint.
package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hrko/dprint-plugin-shfmt/dprint"
	"mvdan.cc/sh/v3/syntax"
)

//go:generate go run github.com/hrko/dprint-plugin-shfmt/dprint/cmd/gen-main-boilerplate -runtime runtime -out main_generated.go

type configuration struct {
	IndentWidth      uint32 `json:"indentWidth"`
	UseTabs          bool   `json:"useTabs"`
	BinaryNextLine   bool   `json:"binaryNextLine"`
	SwitchCaseIndent bool   `json:"switchCaseIndent"`
	SpaceRedirects   bool   `json:"spaceRedirects"`
	KeepPadding      bool   `json:"keepPadding"`
	FuncNextLine     bool   `json:"funcNextLine"`
	Minify           bool   `json:"minify"`
}

type handler struct{}

func (h *handler) ResolveConfig(config dprint.ConfigKeyMap, global dprint.GlobalConfiguration) dprint.ResolveConfigurationResult[configuration] {
	diagnostics := make([]dprint.ConfigurationDiagnostic, 0)
	resolved := configuration{
		IndentWidth:      2,
		UseTabs:          false,
		BinaryNextLine:   false,
		SwitchCaseIndent: false,
		SpaceRedirects:   false,
		KeepPadding:      false,
		FuncNextLine:     false,
		Minify:           false,
	}

	diagnostics = append(diagnostics, unknownPropertyDiagnostics(config)...)

	resolved.IndentWidth = getUInt32(config, "indentWidth", resolved.IndentWidth, &diagnostics)
	resolved.UseTabs = getBool(config, "useTabs", resolved.UseTabs, &diagnostics)
	resolved.BinaryNextLine = getBool(config, "binaryNextLine", resolved.BinaryNextLine, &diagnostics)
	resolved.SwitchCaseIndent = getBool(config, "switchCaseIndent", resolved.SwitchCaseIndent, &diagnostics)
	resolved.SpaceRedirects = getBool(config, "spaceRedirects", resolved.SpaceRedirects, &diagnostics)
	resolved.KeepPadding = getBool(config, "keepPadding", resolved.KeepPadding, &diagnostics)
	resolved.FuncNextLine = getBool(config, "funcNextLine", resolved.FuncNextLine, &diagnostics)
	resolved.Minify = getBool(config, "minify", resolved.Minify, &diagnostics)

	// Global settings override plugin settings for indentation related properties.
	resolved.IndentWidth = getUInt32(global, "indentWidth", resolved.IndentWidth, &diagnostics)
	resolved.UseTabs = getBool(global, "useTabs", resolved.UseTabs, &diagnostics)

	return dprint.ResolveConfigurationResult[configuration]{
		FileMatching: dprint.FileMatchingInfo{
			FileExtensions: []string{"sh", "bash", "zsh", "ksh", "bats"},
			FileNames:      []string{},
		},
		Diagnostics: diagnostics,
		Config:      resolved,
	}
}

func (h *handler) PluginInfo() dprint.PluginInfo {
	return dprint.PluginInfo{
		Name:            "dprint-plugin-shfmt",
		Version:         "0.0.0-dev",
		ConfigKey:       "shfmt",
		HelpURL:         "https://github.com/hrko/dprint-plugin-shfmt",
		ConfigSchemaURL: "",
	}
}

func (h *handler) LicenseText() string {
	return "BSD-3-Clause"
}

func (h *handler) CheckConfigUpdates(_ dprint.CheckConfigUpdatesMessage) ([]dprint.ConfigChange, error) {
	return []dprint.ConfigChange{}, nil
}

func (h *handler) Format(
	request dprint.SyncFormatRequest[configuration],
	_ dprint.HostFormatFunc,
) dprint.FormatResult {
	parser := syntax.NewParser(syntax.Variant(detectVariant(request.FilePath, request.FileBytes)))
	prog, err := parser.Parse(bytes.NewReader(request.FileBytes), request.FilePath)
	if err != nil {
		return dprint.FormatError(err)
	}

	printer := syntax.NewPrinter(
		syntax.Indent(indentSize(request.Config)),
		syntax.BinaryNextLine(request.Config.BinaryNextLine),
		syntax.SwitchCaseIndent(request.Config.SwitchCaseIndent),
		syntax.SpaceRedirects(request.Config.SpaceRedirects),
		syntax.KeepPadding(request.Config.KeepPadding),
		syntax.FunctionNextLine(request.Config.FuncNextLine),
		syntax.Minify(request.Config.Minify),
	)
	var buffer bytes.Buffer
	if err := printer.Print(&buffer, prog); err != nil {
		return dprint.FormatError(err)
	}

	formatted := buffer.Bytes()
	if bytes.Equal(request.FileBytes, formatted) {
		return dprint.NoChange()
	}
	return dprint.Change(append([]byte(nil), formatted...))
}

var runtime = dprint.NewRuntime(&handler{})

func indentSize(config configuration) uint {
	if config.UseTabs {
		return 0
	}
	return uint(config.IndentWidth)
}

func detectVariant(filePath string, fileBytes []byte) syntax.LangVariant {
	if variant, ok := variantFromShebang(fileBytes); ok {
		return variant
	}
	if variant, ok := variantFromFilePath(filePath); ok {
		return variant
	}
	return syntax.LangBash
}

func variantFromFilePath(filePath string) (syntax.LangVariant, bool) {
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(filePath)), ".")
	switch extension {
	case "sh":
		return syntax.LangPOSIX, true
	case "bash", "zsh", "bats":
		return syntax.LangBash, true
	case "ksh", "mksh":
		return syntax.LangMirBSDKorn, true
	default:
		return syntax.LangBash, false
	}
}

func variantFromShebang(fileBytes []byte) (syntax.LangVariant, bool) {
	if len(fileBytes) < 2 || fileBytes[0] != '#' || fileBytes[1] != '!' {
		return syntax.LangBash, false
	}

	lineEnd := bytes.IndexByte(fileBytes, '\n')
	if lineEnd == -1 {
		lineEnd = len(fileBytes)
	}

	shebang := strings.TrimSpace(strings.TrimPrefix(string(fileBytes[:lineEnd]), "#!"))
	fields := strings.Fields(shebang)
	if len(fields) == 0 {
		return syntax.LangBash, false
	}

	interpreter := strings.ToLower(filepath.Base(fields[0]))
	if interpreter == "env" {
		for _, field := range fields[1:] {
			if field == "-S" || strings.HasPrefix(field, "-") {
				continue
			}
			interpreter = strings.ToLower(filepath.Base(field))
			break
		}
	}

	switch interpreter {
	case "sh", "dash", "ash":
		return syntax.LangPOSIX, true
	case "bash", "zsh", "bats":
		return syntax.LangBash, true
	case "ksh", "mksh":
		return syntax.LangMirBSDKorn, true
	default:
		return syntax.LangBash, false
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

func isKnownProperty(key string) bool {
	switch key {
	case "indentWidth", "useTabs", "binaryNextLine", "switchCaseIndent", "spaceRedirects", "keepPadding", "funcNextLine", "minify":
		return true
	default:
		return false
	}
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
