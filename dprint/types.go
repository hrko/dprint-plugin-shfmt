// Package dprint provides minimal types and runtime glue for dprint Wasm plugins.
package dprint

import "errors"

// PluginSchemaVersion is the dprint Wasm plugin schema version supported here.
const PluginSchemaVersion uint32 = 4

// ConfigKeyMap represents plugin-specific configuration as JSON-like data.
type ConfigKeyMap map[string]any

// GlobalConfiguration represents global dprint configuration as JSON-like data.
type GlobalConfiguration map[string]any

// ConfigurationDiagnostic represents a configuration diagnostic object.
type ConfigurationDiagnostic map[string]any

// FormatConfigID identifies a registered configuration.
type FormatConfigID uint32

// FormatConfigIDFromRaw converts a raw numeric config id into FormatConfigID.
func FormatConfigIDFromRaw(raw uint32) FormatConfigID {
	return FormatConfigID(raw)
}

// AsRaw returns this identifier as a raw uint32.
func (id FormatConfigID) AsRaw() uint32 {
	return uint32(id)
}

// RawFormatConfig is the unresolved configuration payload from the host.
type RawFormatConfig struct {
	Plugin ConfigKeyMap        `json:"plugin"`
	Global GlobalConfiguration `json:"global"`
}

// PluginInfo describes plugin metadata exposed to dprint.
type PluginInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	ConfigKey string `json:"configKey"`
	HelpURL   string `json:"helpUrl"`
	// ConfigSchemaURL points to a versioned schema JSON.
	ConfigSchemaURL string `json:"configSchemaUrl"`
	// UpdateURL points to the latest update manifest JSON.
	UpdateURL *string `json:"updateUrl,omitempty"`
}

// FileMatchingInfo declares which files are handled by this plugin.
type FileMatchingInfo struct {
	FileExtensions []string `json:"fileExtensions"`
	FileNames      []string `json:"fileNames"`
}

// CheckConfigUpdatesMessage is the request payload for config update checks.
type CheckConfigUpdatesMessage struct {
	OldVersion *string      `json:"oldVersion,omitempty"`
	Config     ConfigKeyMap `json:"config"`
}

// ConfigChangeKind describes how a config entry should be changed.
type ConfigChangeKind string

// Config change kinds returned by CheckConfigUpdates.
const (
	ConfigChangeKindAdd    ConfigChangeKind = "add"
	ConfigChangeKindSet    ConfigChangeKind = "set"
	ConfigChangeKindRemove ConfigChangeKind = "remove"
)

// ConfigChange describes a single configuration update operation.
type ConfigChange struct {
	Path  []any            `json:"path"`
	Kind  ConfigChangeKind `json:"kind"`
	Value any              `json:"value,omitempty"`
}

// ResolveConfigurationResult contains resolved config and metadata.
type ResolveConfigurationResult[T any] struct {
	FileMatching FileMatchingInfo          `json:"fileMatching"`
	Diagnostics  []ConfigurationDiagnostic `json:"diagnostics"`
	Config       T                         `json:"config"`
}

// FormatRange represents a byte range to format.
type FormatRange struct {
	Start uint32
	End   uint32
}

// CancellationToken reports whether formatting has been cancelled.
type CancellationToken interface {
	IsCancelled() bool
}

// NullCancellationToken never reports cancellation.
type NullCancellationToken struct{}

// IsCancelled always returns false.
func (NullCancellationToken) IsCancelled() bool {
	return false
}

// SyncHostFormatRequest is a request to format via the host.
type SyncHostFormatRequest struct {
	FilePath       string
	FileBytes      []byte
	Range          *FormatRange
	OverrideConfig ConfigKeyMap
}

// SyncFormatRequest is the plugin-facing formatting request payload.
type SyncFormatRequest[T any] struct {
	FilePath  string
	FileBytes []byte
	ConfigID  FormatConfigID
	Config    T
	Range     *FormatRange
	Token     CancellationToken
}

// HostFormatFunc formats text using another plugin via the host.
type HostFormatFunc func(request SyncHostFormatRequest) FormatResult

// FormatResultCode describes the outcome of a format request.
type FormatResultCode uint32

// Format result codes.
const (
	FormatResultNoChange FormatResultCode = 0
	FormatResultChange   FormatResultCode = 1
	FormatResultError    FormatResultCode = 2
)

// FormatResult is the result of a format request.
type FormatResult struct {
	Code FormatResultCode
	Text []byte
	Err  error
}

// NoChange returns a result indicating the input should be kept as-is.
func NoChange() FormatResult {
	return FormatResult{Code: FormatResultNoChange}
}

// Change returns a result containing new formatted text.
func Change(text []byte) FormatResult {
	return FormatResult{
		Code: FormatResultChange,
		Text: text,
	}
}

// FormatError returns a result containing a formatting error.
func FormatError(err error) FormatResult {
	if err == nil {
		err = errors.New("format error")
	}
	return FormatResult{
		Code: FormatResultError,
		Err:  err,
	}
}

// SyncPluginHandler defines the synchronous plugin hooks called by the runtime.
type SyncPluginHandler[T any] interface {
	ResolveConfig(config ConfigKeyMap, global GlobalConfiguration) ResolveConfigurationResult[T]
	PluginInfo() PluginInfo
	LicenseText() string
	CheckConfigUpdates(message CheckConfigUpdatesMessage) ([]ConfigChange, error)
	Format(request SyncFormatRequest[T], formatWithHost HostFormatFunc) FormatResult
}
