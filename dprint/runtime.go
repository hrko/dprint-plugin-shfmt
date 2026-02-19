package dprint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
	"unsafe"
)

type jsonResponse struct {
	Kind string `json:"kind"`
	Data any    `json:"data"`
}

type rawFormatConfigEnvelope struct {
	Plugin map[string]json.RawMessage `json:"plugin"`
	Global map[string]json.RawMessage `json:"global"`
}

type unresolvedConfigEntry struct {
	id     FormatConfigID
	config RawFormatConfig
}

// Runtime manages shared buffers and config lifecycle for a plugin handler.
type Runtime[T any] struct {
	handler SyncPluginHandler[T]

	sharedBytes []byte

	unresolvedConfigs []unresolvedConfigEntry

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
		handler:           handler,
		sharedBytes:       make([]byte, 0),
		unresolvedConfigs: make([]unresolvedConfigEntry, 0),
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
	config, err := parseRawFormatConfig(r.takeSharedBytes())
	if err != nil {
		panic(err)
	}
	if config.Plugin == nil {
		config.Plugin = make(ConfigKeyMap)
	}
	if config.Global == nil {
		config.Global = make(GlobalConfiguration)
	}

	id := FormatConfigIDFromRaw(configID)
	r.setUnresolvedConfig(id, config)
}

// ReleaseConfig removes unresolved and resolved configuration for id.
func (r *Runtime[T]) ReleaseConfig(configID uint32) {
	id := FormatConfigIDFromRaw(configID)
	r.removeUnresolvedConfig(id)
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
	return r.createResolvedConfigResult(configID, nil)
}

func (r *Runtime[T]) createResolvedConfigResult(configID FormatConfigID, overrideConfig ConfigKeyMap) ResolveConfigurationResult[T] {
	unresolvedConfig, ok := r.getUnresolvedConfig(configID)
	if !ok {
		panic(fmt.Sprintf("plugin must have config set before use (id: %d)", configID.AsRaw()))
	}

	pluginConfig := cloneConfigMap(unresolvedConfig.Plugin)
	for key, value := range overrideConfig {
		pluginConfig[key] = value
	}

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
	for key, value := range config {
		newConfig[key] = value
	}
	return newConfig
}

func parseRawFormatConfig(data []byte) (RawFormatConfig, error) {
	var envelope rawFormatConfigEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return RawFormatConfig{}, err
	}

	pluginConfig, err := decodeRawObject(envelope.Plugin)
	if err != nil {
		return RawFormatConfig{}, fmt.Errorf("failed to decode plugin config: %w", err)
	}

	globalConfig, err := decodeRawObject(envelope.Global)
	if err != nil {
		return RawFormatConfig{}, fmt.Errorf("failed to decode global config: %w", err)
	}

	return RawFormatConfig{
		Plugin: pluginConfig,
		Global: GlobalConfiguration(globalConfig),
	}, nil
}

func decodeRawObject(rawMap map[string]json.RawMessage) (map[string]any, error) {
	if rawMap == nil {
		return nil, nil
	}

	result := make(map[string]any, len(rawMap))
	for key, rawValue := range rawMap {
		value, err := decodeRawValue(rawValue)
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", key, err)
		}
		result[key] = value
	}
	return result, nil
}

func decodeRawArray(rawArray []json.RawMessage) ([]any, error) {
	result := make([]any, len(rawArray))
	for i, rawValue := range rawArray {
		value, err := decodeRawValue(rawValue)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", i, err)
		}
		result[i] = value
	}
	return result, nil
}

func decodeRawValue(raw json.RawMessage) (any, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}

	switch trimmed[0] {
	case '{':
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &nested); err != nil {
			return nil, err
		}
		return decodeRawObject(nested)
	case '[':
		var nested []json.RawMessage
		if err := json.Unmarshal(trimmed, &nested); err != nil {
			return nil, err
		}
		return decodeRawArray(nested)
	case '"':
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return nil, err
		}
		return text, nil
	case 't', 'f':
		var boolValue bool
		if err := json.Unmarshal(trimmed, &boolValue); err != nil {
			return nil, err
		}
		return boolValue, nil
	case 'n':
		return nil, nil
	default:
		numberText := string(trimmed)
		if intValue, err := strconv.ParseInt(numberText, 10, 64); err == nil {
			return intValue, nil
		}
		if floatValue, err := strconv.ParseFloat(numberText, 64); err == nil {
			return floatValue, nil
		}
		return nil, fmt.Errorf("unsupported raw value: %q", numberText)
	}
}

func (r *Runtime[T]) setUnresolvedConfig(id FormatConfigID, config RawFormatConfig) {
	for i := range r.unresolvedConfigs {
		if r.unresolvedConfigs[i].id == id {
			r.unresolvedConfigs[i].config = config
			return
		}
	}

	r.unresolvedConfigs = append(r.unresolvedConfigs, unresolvedConfigEntry{
		id:     id,
		config: config,
	})
}

func (r *Runtime[T]) getUnresolvedConfig(id FormatConfigID) (RawFormatConfig, bool) {
	for i := range r.unresolvedConfigs {
		if r.unresolvedConfigs[i].id == id {
			return r.unresolvedConfigs[i].config, true
		}
	}
	return RawFormatConfig{}, false
}

func (r *Runtime[T]) removeUnresolvedConfig(id FormatConfigID) {
	for i := range r.unresolvedConfigs {
		if r.unresolvedConfigs[i].id != id {
			continue
		}

		lastIndex := len(r.unresolvedConfigs) - 1
		r.unresolvedConfigs[i] = r.unresolvedConfigs[lastIndex]
		r.unresolvedConfigs = r.unresolvedConfigs[:lastIndex]
		return
	}
}

type hostCancellationToken struct{}

func (hostCancellationToken) IsCancelled() bool {
	return hostHasCancelled() == 1
}
