package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"unsafe"

	"mvdan.cc/sh/v3/syntax"
)

var sharedBytes []byte
var currentFilePath string
var lastFormattedText string
var lastErrorText string

type pluginConfig struct {
	IndentWidth uint32
	UseTabs     bool
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
	return writeSharedString(`{"name":"dprint-plugin-shfmt","version":"0.1.1","configKey":"shfmt","fileExtensions":["sh","bash","zsh","ksh","bats"],"helpUrl":"https://github.com/dprint/dprint-plugin-shfmt","configSchemaUrl":""}`)
}

//export get_license_text
func get_license_text() uint32 {
	return writeSharedString("dprint-plugin-shfmt: MIT License\nshfmt (mvdan.cc/sh): BSD-3-Clause")
}

//export get_config_file_matching
func get_config_file_matching(configID uint32) uint32 {
	_ = configID
	return writeSharedString(`{"fileExtensions":["sh","bash","zsh","ksh","bats"],"fileNames":[]}`)
}

//export register_config
func register_config(configID uint32) {
	_ = configID
}

//export release_config
func release_config(configID uint32) {
	_ = configID
}

//export get_config_diagnostics
func get_config_diagnostics(configID uint32) uint32 {
	_ = configID
	return writeSharedString("[]")
}

//export get_resolved_config
func get_resolved_config(configID uint32) uint32 {
	_ = configID
	return writeSharedString(`{"indentWidth":2,"useTabs":false}`)
}

//export set_file_path
func set_file_path() {
	currentFilePath = string(sharedBytes)
}

//export set_override_config
func set_override_config() {
	// Ignored in current minimal integration implementation.
}

//export format
func format(configID uint32) uint32 {
	lastFormattedText = ""
	lastErrorText = ""
	_ = configID

	input := string(sharedBytes)
	output, err := formatShell(input, currentFilePath, defaultPluginConfig())
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

func defaultPluginConfig() pluginConfig {
	return pluginConfig{IndentWidth: 2, UseTabs: false}
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

	printer := syntax.NewPrinter(syntax.Indent(indent))

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

func writeSharedBytes(value []byte) uint32 {
	if cap(sharedBytes) < len(value) {
		sharedBytes = make([]byte, len(value))
	} else {
		sharedBytes = sharedBytes[:len(value)]
	}
	copy(sharedBytes, value)
	return uint32(len(sharedBytes))
}
