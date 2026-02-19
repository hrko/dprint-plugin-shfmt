package main

import (
	"encoding/json"
	_ "mvdan.cc/sh/v3/syntax"
	"unsafe"
)

const pluginSchemaVersion uint32 = 4

type rawFormatConfig struct {
	Plugin map[string]any `json:"plugin"`
	Global map[string]any `json:"global"`
}

var sharedBytes []byte

//export dprint_plugin_version_4
func dprint_plugin_version_4() uint32 {
	return pluginSchemaVersion
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

//export get_plugin_info
func get_plugin_info() uint32 {
	return setSharedBytes([]byte(`{"name":"case-syntax-json-unmarshal-only","version":"0.0.0","configKey":"isolation","helpUrl":"https://example.com","configSchemaUrl":""}`))
}

//export get_license_text
func get_license_text() uint32 {
	return setSharedBytes([]byte("MIT"))
}

//export get_config_file_matching
func get_config_file_matching(_ uint32) uint32 {
	return setSharedBytes([]byte(`{"fileExtensions":["sh"],"fileNames":[]}`))
}

//export register_config
func register_config(_ uint32) {
	var cfg rawFormatConfig
	if err := json.Unmarshal(takeSharedBytes(), &cfg); err != nil {
		panic(err)
	}
}

//export release_config
func release_config(_ uint32) {
}

//export get_config_diagnostics
func get_config_diagnostics(_ uint32) uint32 {
	return setSharedBytes([]byte("[]"))
}

//export get_resolved_config
func get_resolved_config(_ uint32) uint32 {
	return setSharedBytes([]byte(`{"indentWidth":2,"useTabs":false}`))
}

//export set_file_path
func set_file_path() {
	_ = takeSharedBytes()
}

//export set_override_config
func set_override_config() {
	_ = takeSharedBytes()
}

//export format
func format(_ uint32) uint32 {
	_ = takeSharedBytes()
	return 0
}

//export format_range
func format_range(configID uint32, _ uint32, _ uint32) uint32 {
	return format(configID)
}

//export get_formatted_text
func get_formatted_text() uint32 {
	return setSharedBytes([]byte{})
}

//export get_error_text
func get_error_text() uint32 {
	return setSharedBytes([]byte{})
}

//export check_config_updates
func check_config_updates() uint32 {
	return setSharedBytes([]byte(`{"kind":"ok","data":[]}`))
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
