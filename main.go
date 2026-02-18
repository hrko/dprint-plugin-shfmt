package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"mvdan.cc/sh/v3/syntax"
)

var sharedBytes []byte

var configStore = map[uint32]resolvedConfigResult{}

var currentFilePath string
var currentOverrideConfig *pluginConfig

var lastFormattedText string
var lastErrorText string

type pluginConfig struct {
	IndentWidth      uint32 `json:"indentWidth"`
	UseTabs          bool   `json:"useTabs"`
	BinaryNextLine   bool   `json:"binaryNextLine"`
	SwitchCaseIndent bool   `json:"switchCaseIndent"`
	SpaceRedirects   bool   `json:"spaceRedirects"`
	KeepPadding      bool   `json:"keepPadding"`
	FuncNextLine     bool   `json:"funcNextLine"`
	Minify           bool   `json:"minify"`
}

type configDiagnostic struct {
	PropertyName string `json:"propertyName"`
	Message      string `json:"message"`
}

type resolvedConfigResult struct {
	Config      pluginConfig       `json:"config"`
	Diagnostics []configDiagnostic `json:"diagnostics"`
}

type registerConfigMessage struct {
	GlobalConfiguration map[string]any `json:"globalConfiguration"`
	PluginConfiguration map[string]any `json:"pluginConfiguration"`
	Config              map[string]any `json:"config"`
}

func main() {}

//export dprint_plugin_version_4
func dprint_plugin_version_4() uint32 {
	return 4
}

//export get_shared_bytes_ptr
func get_shared_bytes_ptr() uint32 {
	if len(sharedBytes) == 0 {
		return 0
	}
	return uint32(uintptr(unsafe.Pointer(&sharedBytes[0])))
}

//export clear_shared_bytes
func clear_shared_bytes(size uint32) uint32 {
	needed := int(size)
	if cap(sharedBytes) < needed {
		sharedBytes = make([]byte, needed)
	} else {
		sharedBytes = sharedBytes[:needed]
		for i := range sharedBytes {
			sharedBytes[i] = 0
		}
	}
	if needed == 0 {
		return 0
	}
	return uint32(uintptr(unsafe.Pointer(&sharedBytes[0])))
}

//export get_plugin_info
func get_plugin_info() uint32 {
	info := map[string]any{
		"name":            "dprint-plugin-shfmt",
		"version":         "0.1.0",
		"configKey":       "shfmt",
		"fileExtensions":  []string{"sh", "bash", "zsh", "ksh", "bats"},
		"helpUrl":         "https://github.com/dprint/dprint-plugin-shfmt",
		"configSchemaUrl": "",
	}
	return writeSharedJSON(info)
}

//export get_license_text
func get_license_text() uint32 {
	licenseText := "dprint-plugin-shfmt: MIT License\nshfmt (mvdan.cc/sh): BSD-3-Clause"
	return writeSharedString(licenseText)
}

//export register_config
func register_config(configID uint32) {
	result := resolveConfigFromSharedBytes()
	configStore[configID] = result
}

//export release_config
func release_config(configID uint32) {
	delete(configStore, configID)
}

//export get_config_diagnostics
func get_config_diagnostics(configID uint32) uint32 {
	result, ok := configStore[configID]
	if !ok {
		return writeSharedJSON([]configDiagnostic{{
			PropertyName: "configId",
			Message:      fmt.Sprintf("unknown config id: %d", configID),
		}})
	}
	return writeSharedJSON(result.Diagnostics)
}

//export get_resolved_config
func get_resolved_config(configID uint32) uint32 {
	result, ok := configStore[configID]
	if !ok {
		return writeSharedJSON(map[string]any{})
	}
	return writeSharedJSON(result.Config)
}

//export set_file_path
func set_file_path() {
	currentFilePath = string(sharedBytes)
}

//export set_override_config
func set_override_config() {
	if len(sharedBytes) == 0 {
		currentOverrideConfig = nil
		return
	}

	var obj map[string]any
	if err := json.Unmarshal(sharedBytes, &obj); err != nil {
		currentOverrideConfig = nil
		return
	}

	resolved := resolvePluginConfig(obj)
	cfg := resolved.Config
	currentOverrideConfig = &cfg
}

//export format
func format(configID uint32) uint32 {
	lastFormattedText = ""
	lastErrorText = ""

	result, ok := configStore[configID]
	if !ok {
		lastErrorText = fmt.Sprintf("unknown config id: %d", configID)
		return 2
	}

	cfg := result.Config
	if currentOverrideConfig != nil {
		cfg = *currentOverrideConfig
	}

	input := string(sharedBytes)
	output, err := formatShell(input, currentFilePath, cfg)
	if err != nil {
		lastErrorText = err.Error()
		return 2
	}
	if output == input {
		return 0
	}

	lastFormattedText = output
	return 1
}

//export get_formatted_text
func get_formatted_text() uint32 {
	return writeSharedString(lastFormattedText)
}

//export get_error_text
func get_error_text() uint32 {
	return writeSharedString(lastErrorText)
}

func resolveConfigFromSharedBytes() resolvedConfigResult {
	if len(sharedBytes) == 0 {
		return resolvePluginConfig(map[string]any{})
	}

	var raw registerConfigMessage
	if err := json.Unmarshal(sharedBytes, &raw); err != nil {
		return resolvedConfigResult{
			Config: defaultPluginConfig(),
			Diagnostics: []configDiagnostic{{
				PropertyName: "config",
				Message:      fmt.Sprintf("invalid config json: %v", err),
			}},
		}
	}

	pluginCfg := raw.PluginConfiguration
	if pluginCfg == nil {
		pluginCfg = raw.Config
	}
	if pluginCfg == nil {
		pluginCfg = map[string]any{}
	}

	return resolvePluginConfig(pluginCfg)
}

func resolvePluginConfig(values map[string]any) resolvedConfigResult {
	cfg := defaultPluginConfig()
	diagnostics := make([]configDiagnostic, 0)

	known := map[string]struct{}{
		"indentWidth":      {},
		"useTabs":          {},
		"binaryNextLine":   {},
		"switchCaseIndent": {},
		"spaceRedirects":   {},
		"keepPadding":      {},
		"funcNextLine":     {},
		"minify":           {},
	}

	for key := range values {
		if _, ok := known[key]; !ok {
			diagnostics = append(diagnostics, configDiagnostic{
				PropertyName: key,
				Message:      "unknown property",
			})
		}
	}

	cfg.IndentWidth = readUint32(values, "indentWidth", cfg.IndentWidth, &diagnostics)
	cfg.UseTabs = readBool(values, "useTabs", cfg.UseTabs, &diagnostics)
	cfg.BinaryNextLine = readBool(values, "binaryNextLine", cfg.BinaryNextLine, &diagnostics)
	cfg.SwitchCaseIndent = readBool(values, "switchCaseIndent", cfg.SwitchCaseIndent, &diagnostics)
	cfg.SpaceRedirects = readBool(values, "spaceRedirects", cfg.SpaceRedirects, &diagnostics)
	cfg.KeepPadding = readBool(values, "keepPadding", cfg.KeepPadding, &diagnostics)
	cfg.FuncNextLine = readBool(values, "funcNextLine", cfg.FuncNextLine, &diagnostics)
	cfg.Minify = readBool(values, "minify", cfg.Minify, &diagnostics)

	return resolvedConfigResult{Config: cfg, Diagnostics: diagnostics}
}

func defaultPluginConfig() pluginConfig {
	return pluginConfig{
		IndentWidth:      2,
		UseTabs:          false,
		BinaryNextLine:   false,
		SwitchCaseIndent: false,
		SpaceRedirects:   false,
		KeepPadding:      false,
		FuncNextLine:     false,
		Minify:           false,
	}
}

func readUint32(values map[string]any, key string, defaultValue uint32, diagnostics *[]configDiagnostic) uint32 {
	value, ok := values[key]
	if !ok {
		return defaultValue
	}

	floatVal, ok := value.(float64)
	if !ok || floatVal < 0 {
		*diagnostics = append(*diagnostics, configDiagnostic{
			PropertyName: key,
			Message:      "expected a non-negative number",
		})
		return defaultValue
	}

	return uint32(floatVal)
}

func readBool(values map[string]any, key string, defaultValue bool, diagnostics *[]configDiagnostic) bool {
	value, ok := values[key]
	if !ok {
		return defaultValue
	}

	boolVal, ok := value.(bool)
	if !ok {
		*diagnostics = append(*diagnostics, configDiagnostic{
			PropertyName: key,
			Message:      "expected a boolean",
		})
		return defaultValue
	}

	return boolVal
}

func formatShell(input string, filePath string, cfg pluginConfig) (string, error) {
	lang := detectLang(filePath, input)
	parser := syntax.NewParser(syntax.Variant(lang))
	file, err := parser.Parse(strings.NewReader(input), "")
	if err != nil {
		return "", err
	}

	indent := uint(cfg.IndentWidth)
	if cfg.UseTabs {
		indent = 0
	}

	printer := syntax.NewPrinter(
		syntax.Indent(indent),
		syntax.BinaryNextLine(cfg.BinaryNextLine),
		syntax.SwitchCaseIndent(cfg.SwitchCaseIndent),
		syntax.SpaceRedirects(cfg.SpaceRedirects),
		syntax.KeepPadding(cfg.KeepPadding),
		syntax.FunctionNextLine(cfg.FuncNextLine),
		syntax.Minify(cfg.Minify),
	)

	var buf bytes.Buffer
	if err := printer.Print(&buf, file); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func detectLang(filePath string, input string) syntax.LangVariant {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filePath), "."))
	switch ext {
	case "sh", "bats":
		return syntax.LangPOSIX
	case "bash":
		return syntax.LangBash
	case "zsh", "ksh":
		return syntax.LangMirBSDKorn
	}

	if strings.HasPrefix(input, "#!/") {
		firstLine := input
		if idx := strings.IndexByte(input, '\n'); idx >= 0 {
			firstLine = input[:idx]
		}
		shebang := strings.ToLower(firstLine)
		switch {
		case strings.Contains(shebang, "bash"):
			return syntax.LangBash
		case strings.Contains(shebang, "zsh"), strings.Contains(shebang, "ksh"):
			return syntax.LangMirBSDKorn
		}
	}

	return syntax.LangPOSIX
}

func writeSharedString(text string) uint32 {
	return writeSharedBytes([]byte(text))
}

func writeSharedJSON(value any) uint32 {
	bytesValue, err := json.Marshal(value)
	if err != nil {
		return writeSharedString("[]")
	}
	return writeSharedBytes(bytesValue)
}

func writeSharedBytes(value []byte) uint32 {
	if cap(sharedBytes) < len(value) {
		sharedBytes = make([]byte, len(value))
	} else {
		sharedBytes = sharedBytes[:len(value)]
	}
	copy(sharedBytes, value)
	return uint32(len(sharedBytes))
}
