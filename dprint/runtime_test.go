package dprint

import (
	"encoding/json"
	"errors"
	"testing"
)

type testConfig struct {
	Value int `json:"value"`
}

type testHandler struct {
	resolveConfigCallCount int

	nextFormatResult FormatResult

	lastFormatRequest SyncFormatRequest[testConfig]

	checkConfigUpdatesChanges []ConfigChange
	checkConfigUpdatesErr     error
}

func (h *testHandler) ResolveConfig(config ConfigKeyMap, _ GlobalConfiguration) ResolveConfigurationResult[testConfig] {
	h.resolveConfigCallCount++

	return ResolveConfigurationResult[testConfig]{
		FileMatching: FileMatchingInfo{
			FileExtensions: []string{"sh"},
			FileNames:      []string{},
		},
		Diagnostics: []ConfigurationDiagnostic{},
		Config: testConfig{
			Value: getInt(config["value"]),
		},
	}
}

func (h *testHandler) PluginInfo() PluginInfo {
	return PluginInfo{
		Name:            "dprint-plugin-shfmt",
		Version:         "0.0.0",
		ConfigKey:       "shfmt",
		HelpURL:         "https://example.com",
		ConfigSchemaURL: "",
	}
}

func (h *testHandler) LicenseText() string {
	return "license"
}

func (h *testHandler) CheckConfigUpdates(_ CheckConfigUpdatesMessage) ([]ConfigChange, error) {
	if h.checkConfigUpdatesErr != nil {
		return nil, h.checkConfigUpdatesErr
	}
	return h.checkConfigUpdatesChanges, nil
}

func (h *testHandler) Format(request SyncFormatRequest[testConfig], _ HostFormatFunc) FormatResult {
	h.lastFormatRequest = request
	return h.nextFormatResult
}

func TestConfigLifecycleAndResolvedConfigCaching(t *testing.T) {
	handler := &testHandler{}
	runtime := NewRuntime[testConfig](handler)

	runtime.sharedBytes = []byte(`{"plugin":{"value":1},"global":{"lineWidth":120}}`)
	runtime.RegisterConfig(1)

	runtime.GetResolvedConfig(1)
	var resolvedConfig testConfig
	if err := json.Unmarshal(runtime.sharedBytes, &resolvedConfig); err != nil {
		t.Fatal(err)
	}
	if resolvedConfig.Value != 1 {
		t.Fatalf("expected resolved config value to be 1, got %d", resolvedConfig.Value)
	}

	runtime.GetResolvedConfig(1)
	if handler.resolveConfigCallCount != 1 {
		t.Fatalf("expected ResolveConfig to be called once due to caching, got %d", handler.resolveConfigCallCount)
	}

	runtime.ReleaseConfig(1)

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when using released config")
		}
	}()
	runtime.GetResolvedConfig(1)
}

func TestFormatFlowAndPathNormalization(t *testing.T) {
	handler := &testHandler{
		nextFormatResult: Change([]byte("formatted")),
	}
	runtime := NewRuntime[testConfig](handler)

	runtime.sharedBytes = []byte(`{"plugin":{"value":2},"global":{}}`)
	runtime.RegisterConfig(1)

	runtime.sharedBytes = []byte(`C:\repo\script.sh`)
	runtime.SetFilePath()
	runtime.sharedBytes = []byte("echo test")

	result := runtime.Format(1)
	if result != uint32(FormatResultChange) {
		t.Fatalf("expected format result %d, got %d", FormatResultChange, result)
	}
	if handler.lastFormatRequest.FilePath != "C:/repo/script.sh" {
		t.Fatalf("expected normalized path, got %q", handler.lastFormatRequest.FilePath)
	}

	runtime.GetFormattedText()
	if string(runtime.sharedBytes) != "formatted" {
		t.Fatalf("expected formatted text to be returned, got %q", string(runtime.sharedBytes))
	}

	handler.nextFormatResult = FormatError(errors.New("format failed"))
	runtime.sharedBytes = []byte("script.sh")
	runtime.SetFilePath()
	runtime.sharedBytes = []byte("echo test")
	result = runtime.Format(1)
	if result != uint32(FormatResultError) {
		t.Fatalf("expected format result %d, got %d", FormatResultError, result)
	}
	runtime.GetErrorText()
	if string(runtime.sharedBytes) != "format failed" {
		t.Fatalf("expected error text, got %q", string(runtime.sharedBytes))
	}

	handler.nextFormatResult = NoChange()
	runtime.sharedBytes = []byte("script.sh")
	runtime.SetFilePath()
	runtime.sharedBytes = []byte("echo test")
	result = runtime.Format(1)
	if result != uint32(FormatResultNoChange) {
		t.Fatalf("expected format result %d, got %d", FormatResultNoChange, result)
	}
}

func TestCheckConfigUpdatesResponse(t *testing.T) {
	handler := &testHandler{
		checkConfigUpdatesChanges: []ConfigChange{
			{
				Path:  []any{"indentWidth"},
				Kind:  ConfigChangeKindSet,
				Value: 2,
			},
		},
	}
	runtime := NewRuntime[testConfig](handler)

	runtime.sharedBytes = []byte(`{"config":{"indentWidth":4}}`)
	runtime.CheckConfigUpdates()

	var okResponse struct {
		Kind string          `json:"kind"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(runtime.sharedBytes, &okResponse); err != nil {
		t.Fatal(err)
	}
	if okResponse.Kind != "ok" {
		t.Fatalf("expected ok response, got %q", okResponse.Kind)
	}

	handler.checkConfigUpdatesErr = errors.New("update failed")
	runtime.sharedBytes = []byte(`{"config":{"indentWidth":4}}`)
	runtime.CheckConfigUpdates()

	var errResponse struct {
		Kind string `json:"kind"`
		Data string `json:"data"`
	}
	if err := json.Unmarshal(runtime.sharedBytes, &errResponse); err != nil {
		t.Fatal(err)
	}
	if errResponse.Kind != "err" {
		t.Fatalf("expected err response, got %q", errResponse.Kind)
	}
	if errResponse.Data != "update failed" {
		t.Fatalf("expected error message, got %q", errResponse.Data)
	}

	runtime.sharedBytes = []byte(`{"config":`)
	runtime.CheckConfigUpdates()
	if err := json.Unmarshal(runtime.sharedBytes, &errResponse); err != nil {
		t.Fatal(err)
	}
	if errResponse.Kind != "err" {
		t.Fatalf("expected err response for invalid json, got %q", errResponse.Kind)
	}
}

func getInt(value any) int {
	switch value := value.(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}
