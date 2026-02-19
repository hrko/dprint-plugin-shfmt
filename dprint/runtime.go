package dprint

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"unicode/utf8"
	"unsafe"
)

type jsonResponse struct {
	Kind string `json:"kind"`
	Data any    `json:"data"`
}

// Runtime manages shared buffers and config lifecycle for a plugin handler.
type Runtime[T any] struct {
	handler SyncPluginHandler[T]

	sharedBytes []byte

	unresolvedConfig map[FormatConfigID]RawFormatConfig
	resolvedConfig   map[FormatConfigID]ResolveConfigurationResult[T]

	overrideConfig *ConfigKeyMap
	filePath       *string

	formattedText    []byte
	hasFormattedText bool
	errorText        string
	hasErrorText     bool
}

// NewRuntime creates a runtime for the provided synchronous plugin handler.
func NewRuntime[T any](handler SyncPluginHandler[T]) *Runtime[T] {
	if handler == nil {
		panic("handler is required")
	}

	return &Runtime[T]{
		handler:          handler,
		sharedBytes:      make([]byte, 0),
		unresolvedConfig: make(map[FormatConfigID]RawFormatConfig),
		resolvedConfig:   make(map[FormatConfigID]ResolveConfigurationResult[T]),
	}
}

// DprintPluginVersion4 returns the schema version supported by this runtime.
func (r *Runtime[T]) DprintPluginVersion4() uint32 {
	return PluginSchemaVersion
}

// GetPluginInfo writes plugin metadata JSON to shared bytes and returns its length.
func (r *Runtime[T]) GetPluginInfo() uint32 {
	bytes, err := json.Marshal(r.handler.PluginInfo())
	if err != nil {
		panic(err)
	}
	return r.setSharedBytes(bytes)
}

// GetLicenseText writes license text to shared bytes and returns its length.
func (r *Runtime[T]) GetLicenseText() uint32 {
	return r.setSharedBytes([]byte(r.handler.LicenseText()))
}

// RegisterConfig stores unresolved configuration received through shared bytes.
func (r *Runtime[T]) RegisterConfig(configID uint32) {
	var config RawFormatConfig
	if err := json.Unmarshal(r.takeSharedBytes(), &config); err != nil {
		panic(err)
	}
	if config.Plugin == nil {
		config.Plugin = make(ConfigKeyMap)
	}
	if config.Global == nil {
		config.Global = make(GlobalConfiguration)
	}

	id := FormatConfigIDFromRaw(configID)
	r.unresolvedConfig[id] = config
	delete(r.resolvedConfig, id)
}

// ReleaseConfig removes unresolved and resolved configuration for id.
func (r *Runtime[T]) ReleaseConfig(configID uint32) {
	id := FormatConfigIDFromRaw(configID)
	delete(r.unresolvedConfig, id)
	delete(r.resolvedConfig, id)
}

// GetConfigDiagnostics writes diagnostics JSON for id and returns its length.
func (r *Runtime[T]) GetConfigDiagnostics(configID uint32) uint32 {
	resolved := r.getResolvedConfigResult(FormatConfigIDFromRaw(configID))
	bytes, err := json.Marshal(resolved.Diagnostics)
	if err != nil {
		panic(err)
	}
	return r.setSharedBytes(bytes)
}

// GetResolvedConfig writes resolved config JSON for id and returns its length.
func (r *Runtime[T]) GetResolvedConfig(configID uint32) uint32 {
	resolved := r.getResolvedConfigResult(FormatConfigIDFromRaw(configID))
	bytes, err := json.Marshal(resolved.Config)
	if err != nil {
		panic(err)
	}
	return r.setSharedBytes(bytes)
}

// GetConfigFileMatching writes file matching JSON for id and returns its length.
func (r *Runtime[T]) GetConfigFileMatching(configID uint32) uint32 {
	resolved := r.getResolvedConfigResult(FormatConfigIDFromRaw(configID))
	bytes, err := json.Marshal(resolved.FileMatching)
	if err != nil {
		panic(err)
	}
	return r.setSharedBytes(bytes)
}

// SetOverrideConfig stores one-off override config from shared bytes.
func (r *Runtime[T]) SetOverrideConfig() {
	var config *ConfigKeyMap
	if err := json.Unmarshal(r.takeSharedBytes(), &config); err != nil {
		panic(err)
	}
	r.overrideConfig = config
}

// SetFilePath stores and normalizes a file path from shared bytes.
func (r *Runtime[T]) SetFilePath() {
	filePathBytes := r.takeSharedBytes()
	if !utf8.Valid(filePathBytes) {
		panic("expected file path to be utf-8")
	}
	pathText := strings.ReplaceAll(string(filePathBytes), "\\", "/")
	r.filePath = &pathText
}

// Format formats full file contents for the specified config id.
func (r *Runtime[T]) Format(configID uint32) uint32 {
	return r.formatInner(FormatConfigIDFromRaw(configID), nil)
}

// FormatRange formats only the specified byte range for the config id.
func (r *Runtime[T]) FormatRange(configID uint32, rangeStart uint32, rangeEnd uint32) uint32 {
	return r.formatInner(FormatConfigIDFromRaw(configID), &FormatRange{
		Start: rangeStart,
		End:   rangeEnd,
	})
}

// GetFormattedText writes the most recent formatted text and returns its length.
func (r *Runtime[T]) GetFormattedText() uint32 {
	if !r.hasFormattedText {
		panic("expected to have formatted text")
	}

	text := r.formattedText
	r.formattedText = nil
	r.hasFormattedText = false

	return r.setSharedBytes(text)
}

// GetErrorText writes the most recent format error text and returns its length.
func (r *Runtime[T]) GetErrorText() uint32 {
	if !r.hasErrorText {
		panic("expected to have error text")
	}

	text := r.errorText
	r.errorText = ""
	r.hasErrorText = false

	return r.setSharedBytes([]byte(text))
}

// CheckConfigUpdates writes check-config-updates response JSON and returns its length.
func (r *Runtime[T]) CheckConfigUpdates() uint32 {
	var response jsonResponse

	var message CheckConfigUpdatesMessage
	if err := json.Unmarshal(r.takeSharedBytes(), &message); err != nil {
		response = jsonResponse{
			Kind: "err",
			Data: err.Error(),
		}
	} else {
		changes, err := r.handler.CheckConfigUpdates(message)
		if err != nil {
			response = jsonResponse{
				Kind: "err",
				Data: err.Error(),
			}
		} else {
			response = jsonResponse{
				Kind: "ok",
				Data: changes,
			}
		}
	}

	bytes, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}

	return r.setSharedBytes(bytes)
}

// GetSharedBytesPtr returns a pointer to the shared byte buffer.
func (r *Runtime[T]) GetSharedBytesPtr() uint32 {
	return bytesPtr(r.sharedBytes)
}

// ClearSharedBytes resizes and clears shared bytes, then returns its pointer.
func (r *Runtime[T]) ClearSharedBytes(size uint32) uint32 {
	intSize := int(size)
	if cap(r.sharedBytes) >= intSize {
		r.sharedBytes = r.sharedBytes[:intSize]
		clear(r.sharedBytes)
	} else {
		r.sharedBytes = make([]byte, intSize)
	}
	return bytesPtr(r.sharedBytes)
}

func (r *Runtime[T]) formatInner(configID FormatConfigID, formatRange *FormatRange) uint32 {
	var resolvedConfig ResolveConfigurationResult[T]

	if r.overrideConfig != nil {
		resolvedConfig = r.createResolvedConfigResult(configID, *r.overrideConfig)
		r.overrideConfig = nil
	} else {
		resolvedConfig = r.getResolvedConfigResult(configID)
	}

	if r.filePath == nil {
		panic("expected the file path to be set")
	}
	filePath := *r.filePath
	r.filePath = nil

	result := r.handler.Format(
		SyncFormatRequest[T]{
			FilePath:  filePath,
			FileBytes: r.takeSharedBytes(),
			ConfigID:  configID,
			Config:    resolvedConfig.Config,
			Range:     formatRange,
			Token:     hostCancellationToken{},
		},
		r.formatWithHost,
	)

	switch result.Code {
	case FormatResultNoChange:
		return uint32(FormatResultNoChange)
	case FormatResultChange:
		r.formattedText = result.Text
		r.hasFormattedText = true
		return uint32(FormatResultChange)
	case FormatResultError:
		if result.Err == nil {
			panic("format error result requires an error message")
		}
		r.errorText = result.Err.Error()
		r.hasErrorText = true
		return uint32(FormatResultError)
	default:
		panic(fmt.Sprintf("unknown format result code: %d", result.Code))
	}
}

func (r *Runtime[T]) getResolvedConfigResult(configID FormatConfigID) ResolveConfigurationResult[T] {
	if resolved, ok := r.resolvedConfig[configID]; ok {
		return resolved
	}

	resolved := r.createResolvedConfigResult(configID, nil)
	r.resolvedConfig[configID] = resolved
	return resolved
}

func (r *Runtime[T]) createResolvedConfigResult(configID FormatConfigID, overrideConfig ConfigKeyMap) ResolveConfigurationResult[T] {
	unresolvedConfig, ok := r.unresolvedConfig[configID]
	if !ok {
		panic(fmt.Sprintf("plugin must have config set before use (id: %d)", configID.AsRaw()))
	}

	pluginConfig := cloneConfigMap(unresolvedConfig.Plugin)
	maps.Copy(pluginConfig, overrideConfig)

	result := r.handler.ResolveConfig(pluginConfig, unresolvedConfig.Global)
	if result.Diagnostics == nil {
		result.Diagnostics = make([]ConfigurationDiagnostic, 0)
	}
	if result.FileMatching.FileExtensions == nil {
		result.FileMatching.FileExtensions = make([]string, 0)
	}
	if result.FileMatching.FileNames == nil {
		result.FileMatching.FileNames = make([]string, 0)
	}

	return result
}

func (r *Runtime[T]) formatWithHost(request SyncHostFormatRequest) FormatResult {
	overrideConfigBytes := []byte{}
	if len(request.OverrideConfig) > 0 {
		bytes, err := json.Marshal(request.OverrideConfig)
		if err != nil {
			return FormatError(err)
		}
		overrideConfigBytes = bytes
	}

	startRange := uint32(0)
	endRange := uint32(len(request.FileBytes))
	if request.Range != nil {
		startRange = request.Range.Start
		endRange = request.Range.End
	}

	filePathBytes := []byte(request.FilePath)
	resultCode := hostFormat(
		bytesPtr(filePathBytes),
		uint32(len(filePathBytes)),
		startRange,
		endRange,
		bytesPtr(overrideConfigBytes),
		uint32(len(overrideConfigBytes)),
		bytesPtr(request.FileBytes),
		uint32(len(request.FileBytes)),
	)

	switch resultCode {
	case uint32(FormatResultNoChange):
		return NoChange()
	case uint32(FormatResultChange):
		textLength := hostGetFormattedText()
		return Change(r.readBytesFromHost(textLength))
	case uint32(FormatResultError):
		errLength := hostGetErrorText()
		return FormatError(fmt.Errorf("%s", string(r.readBytesFromHost(errLength))))
	default:
		panic(fmt.Sprintf("unknown host format value: %d", resultCode))
	}
}

func (r *Runtime[T]) readBytesFromHost(length uint32) []byte {
	ptr := r.ClearSharedBytes(length)
	hostWriteBuffer(ptr)
	return r.takeSharedBytes()
}

func (r *Runtime[T]) takeSharedBytes() []byte {
	bytes := r.sharedBytes
	r.sharedBytes = nil
	return bytes
}

func (r *Runtime[T]) setSharedBytes(bytes []byte) uint32 {
	r.sharedBytes = bytes
	return uint32(len(bytes))
}

func bytesPtr(bytes []byte) uint32 {
	if len(bytes) == 0 {
		return 0
	}
	return uint32(uintptr(unsafe.Pointer(&bytes[0])))
}

func cloneConfigMap(config ConfigKeyMap) ConfigKeyMap {
	if len(config) == 0 {
		return make(ConfigKeyMap)
	}

	newConfig := make(ConfigKeyMap, len(config))
	maps.Copy(newConfig, config)
	return newConfig
}

type hostCancellationToken struct{}

func (hostCancellationToken) IsCancelled() bool {
	return hostHasCancelled() == 1
}
