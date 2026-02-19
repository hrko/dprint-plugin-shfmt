package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
	"unsafe"
)

const pluginSchemaVersion uint32 = 4

type rawFormatConfig struct {
	Plugin map[string]any `json:"plugin"`
	Global map[string]any `json:"global"`
}

type pluginInfo struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	ConfigKey       string `json:"configKey"`
	HelpURL         string `json:"helpUrl"`
	ConfigSchemaURL string `json:"configSchemaUrl"`
}

var sharedBytes []byte
var unresolvedConfig = map[uint32]rawFormatConfig{}
var overrideConfig *map[string]any
var filePath *string
var formattedText []byte
var hasFormattedText bool
var errorText string
var hasErrorText bool

//export dprint_plugin_version_4
func dprint_plugin_version_4() uint32 {
	return pluginSchemaVersion
}

//export register_config
func register_config(configID uint32) {
	var config rawFormatConfig
	if err := json.Unmarshal(takeSharedBytes(), &config); err != nil {
		panic(err)
	}
	if config.Plugin == nil {
		config.Plugin = map[string]any{}
	}
	if config.Global == nil {
		config.Global = map[string]any{}
	}
	unresolvedConfig[configID] = config
}

//export release_config
func release_config(configID uint32) {
	delete(unresolvedConfig, configID)
}

//export get_config_diagnostics
func get_config_diagnostics(configID uint32) uint32 {
	_, ok := unresolvedConfig[configID]
	if !ok {
		panic(fmt.Sprintf("config not found: %d", configID))
	}
	return setSharedBytes([]byte("[]"))
}

//export get_resolved_config
func get_resolved_config(configID uint32) uint32 {
	config, ok := unresolvedConfig[configID]
	if !ok {
		panic(fmt.Sprintf("config not found: %d", configID))
	}

	cloned := cloneConfigMap(config.Plugin)
	if overrideConfig != nil {
		for key, value := range *overrideConfig {
			cloned[key] = value
		}
		overrideConfig = nil
	}

	bytes, err := json.Marshal(cloned)
	if err != nil {
		panic(err)
	}
	return setSharedBytes(bytes)
}

//export get_config_file_matching
func get_config_file_matching(configID uint32) uint32 {
	_, ok := unresolvedConfig[configID]
	if !ok {
		panic(fmt.Sprintf("config not found: %d", configID))
	}
	return setSharedBytes([]byte(`{"fileExtensions":["sh"],"fileNames":[]}`))
}

//export get_license_text
func get_license_text() uint32 {
	return setSharedBytes([]byte("MIT"))
}

//export get_plugin_info
func get_plugin_info() uint32 {
	bytes, err := json.Marshal(pluginInfo{
		Name:            "isolation-plugin-no-maps",
		Version:         "0.0.0",
		ConfigKey:       "isolation",
		HelpURL:         "https://example.com",
		ConfigSchemaURL: "",
	})
	if err != nil {
		panic(err)
	}
	return setSharedBytes(bytes)
}

//export set_file_path
func set_file_path() {
	filePathBytes := takeSharedBytes()
	if !utf8.Valid(filePathBytes) {
		panic("expected file path to be utf-8")
	}
	path := strings.ReplaceAll(string(filePathBytes), "\\", "/")
	filePath = &path
}

//export set_override_config
func set_override_config() {
	var config *map[string]any
	if err := json.Unmarshal(takeSharedBytes(), &config); err != nil {
		panic(err)
	}
	overrideConfig = config
}

//export format
func format(_ uint32) uint32 {
	filePath = nil
	_ = takeSharedBytes()
	return 0
}

//export format_range
func format_range(configID uint32, _ uint32, _ uint32) uint32 {
	return format(configID)
}

//export get_formatted_text
func get_formatted_text() uint32 {
	if !hasFormattedText {
		panic("expected formatted text")
	}
	text := formattedText
	formattedText = nil
	hasFormattedText = false
	return setSharedBytes(text)
}

//export get_error_text
func get_error_text() uint32 {
	if !hasErrorText {
		panic("expected error text")
	}
	text := errorText
	errorText = ""
	hasErrorText = false
	return setSharedBytes([]byte(text))
}

//export check_config_updates
func check_config_updates() uint32 {
	return setSharedBytes([]byte(`{"kind":"ok","data":[]}`))
}

//export get_shared_bytes_ptr
func get_shared_bytes_ptr() uint32 {
	return bytesPtr(sharedBytes)
}

//export clear_shared_bytes
func clear_shared_bytes(size uint32) uint32 {
	intSize := int(size)
	if cap(sharedBytes) >= intSize {
		sharedBytes = sharedBytes[:intSize]
		clear(sharedBytes)
	} else {
		sharedBytes = make([]byte, intSize)
	}
	return bytesPtr(sharedBytes)
}

func cloneConfigMap(config map[string]any) map[string]any {
	if len(config) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(config))
	for key, value := range config {
		cloned[key] = value
	}
	return cloned
}

func takeSharedBytes() []byte {
	bytes := sharedBytes
	sharedBytes = nil
	return bytes
}

func setSharedBytes(bytes []byte) uint32 {
	sharedBytes = bytes
	return uint32(len(bytes))
}

func bytesPtr(bytes []byte) uint32 {
	if len(bytes) == 0 {
		return 0
	}
	return uint32(uintptr(unsafe.Pointer(&bytes[0])))
}

func main() {}
