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

	lastFormatRequest  SyncFormatRequest[testConfig]
	lastTokenCancelled bool

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
	h.lastTokenCancelled = request.Token.IsCancelled()
	return h.nextFormatResult
}

type testHostBridge struct {
	formatResultCode uint32
	formattedText    []byte
	errorText        []byte
	cancelled        bool

	formatCalled bool

	gotRequest hostFormatRequest
}

func (h *testHostBridge) writeBuffer(_ uint32) {
}

func (h *testHostBridge) format(request hostFormatRequest) uint32 {
	h.formatCalled = true
	h.gotRequest = request
	return h.formatResultCode
}

func (h *testHostBridge) readFormattedText(_ func(length uint32) []byte) []byte {
	return append([]byte(nil), h.formattedText...)
}

func (h *testHostBridge) readErrorText(_ func(length uint32) []byte) string {
	return string(h.errorText)
}

func (h *testHostBridge) hasCancelled() bool {
	return h.cancelled
}

func TestConfigLifecycleAndResolvedConfigResolution(t *testing.T) {
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
	if handler.resolveConfigCallCount != 2 {
		t.Fatalf("expected ResolveConfig to be called for each request, got %d", handler.resolveConfigCallCount)
	}

	runtime.ReleaseConfig(1)

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when using released config")
		}
	}()
	runtime.GetResolvedConfig(1)
}

func TestConfigLifecycleMultipleIDs(t *testing.T) {
	handler := &testHandler{}
	runtime := NewRuntime[testConfig](handler)

	runtime.sharedBytes = []byte(`{"plugin":{"value":1},"global":{}}`)
	runtime.RegisterConfig(1)
	runtime.sharedBytes = []byte(`{"plugin":{"value":2},"global":{}}`)
	runtime.RegisterConfig(2)

	runtime.GetResolvedConfig(1)
	var resolvedConfig testConfig
	if err := json.Unmarshal(runtime.sharedBytes, &resolvedConfig); err != nil {
		t.Fatal(err)
	}
	if resolvedConfig.Value != 1 {
		t.Fatalf("expected resolved config value to be 1, got %d", resolvedConfig.Value)
	}

	runtime.GetResolvedConfig(2)
	if err := json.Unmarshal(runtime.sharedBytes, &resolvedConfig); err != nil {
		t.Fatal(err)
	}
	if resolvedConfig.Value != 2 {
		t.Fatalf("expected resolved config value to be 2, got %d", resolvedConfig.Value)
	}

	runtime.ReleaseConfig(1)

	runtime.GetResolvedConfig(2)
	if err := json.Unmarshal(runtime.sharedBytes, &resolvedConfig); err != nil {
		t.Fatal(err)
	}
	if resolvedConfig.Value != 2 {
		t.Fatalf("expected resolved config value to remain 2 after releasing config 1, got %d", resolvedConfig.Value)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when using released config id")
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

func TestParseRawFormatConfigDecodesPrimitiveValues(t *testing.T) {
	config, err := parseRawFormatConfig([]byte(`{
		"plugin": {
			"indentWidth": 2,
			"useTabs": false,
			"name": "demo"
		},
		"global": {
			"lineWidth": 120
		}
	}`))
	if err != nil {
		t.Fatalf("expected parse to succeed: %v", err)
	}

	if getInt(config.Plugin["indentWidth"]) != 2 {
		t.Fatalf("expected plugin indentWidth 2, got %#v", config.Plugin["indentWidth"])
	}

	useTabs, ok := config.Plugin["useTabs"].(bool)
	if !ok || useTabs {
		t.Fatalf("expected plugin useTabs false, got %#v", config.Plugin["useTabs"])
	}

	if config.Plugin["name"] != "demo" {
		t.Fatalf("expected plugin name demo, got %#v", config.Plugin["name"])
	}

	if getInt(config.Global["lineWidth"]) != 120 {
		t.Fatalf("expected global lineWidth 120, got %#v", config.Global["lineWidth"])
	}
}

func TestFormatWithHostNoChangeForwardsRequest(t *testing.T) {
	runtime := NewRuntime[testConfig](&testHandler{})
	host := &testHostBridge{
		formatResultCode: uint32(FormatResultNoChange),
	}
	runtime.host = host

	result := runtime.formatWithHost(SyncHostFormatRequest{
		FilePath:  "script.sh",
		FileBytes: []byte("echo test\n"),
		Range: &FormatRange{
			Start: 2,
			End:   5,
		},
		OverrideConfig: ConfigKeyMap{
			"useTabs": true,
		},
	})

	if result.Code != FormatResultNoChange {
		t.Fatalf("expected no-change result, got %d", result.Code)
	}
	if !host.formatCalled {
		t.Fatal("expected host format to be called")
	}
	if host.gotRequest.filePath != "script.sh" {
		t.Fatalf("expected forwarded file path, got %q", host.gotRequest.filePath)
	}
	if host.gotRequest.rangeStart != 2 || host.gotRequest.rangeEnd != 5 {
		t.Fatalf("expected forwarded range 2..5, got %d..%d", host.gotRequest.rangeStart, host.gotRequest.rangeEnd)
	}
	if string(host.gotRequest.fileBytes) != "echo test\n" {
		t.Fatalf("expected forwarded file bytes, got %q", string(host.gotRequest.fileBytes))
	}
	if len(host.gotRequest.overrideConfig) == 0 {
		t.Fatal("expected override config to be forwarded")
	}
}

func TestFormatWithHostChangeReadsFormattedBytes(t *testing.T) {
	runtime := NewRuntime[testConfig](&testHandler{})
	host := &testHostBridge{
		formatResultCode: uint32(FormatResultChange),
		formattedText:    []byte("formatted\n"),
	}
	runtime.host = host

	result := runtime.formatWithHost(SyncHostFormatRequest{
		FilePath:  "script.sh",
		FileBytes: []byte("echo test\n"),
	})

	if result.Code != FormatResultChange {
		t.Fatalf("expected change result, got %d", result.Code)
	}
	if string(result.Text) != "formatted\n" {
		t.Fatalf("expected formatted text, got %q", string(result.Text))
	}
	if host.gotRequest.rangeStart != 0 || host.gotRequest.rangeEnd != uint32(len("echo test\n")) {
		t.Fatalf("expected default range to cover input, got %d..%d", host.gotRequest.rangeStart, host.gotRequest.rangeEnd)
	}
}

func TestFormatWithHostErrorReadsErrorBytes(t *testing.T) {
	runtime := NewRuntime[testConfig](&testHandler{})
	host := &testHostBridge{
		formatResultCode: uint32(FormatResultError),
		errorText:        []byte("host failed"),
	}
	runtime.host = host

	result := runtime.formatWithHost(SyncHostFormatRequest{
		FilePath:  "script.sh",
		FileBytes: []byte("echo test\n"),
	})

	if result.Code != FormatResultError {
		t.Fatalf("expected error result, got %d", result.Code)
	}
	if result.Err == nil || result.Err.Error() != "host failed" {
		t.Fatalf("expected host error text, got %v", result.Err)
	}
}

func TestFormatPassesCancellationTokenFromHost(t *testing.T) {
	handler := &testHandler{
		nextFormatResult: NoChange(),
	}
	runtime := NewRuntime[testConfig](handler)
	runtime.host = &testHostBridge{cancelled: true}

	runtime.sharedBytes = []byte(`{"plugin":{"value":2},"global":{}}`)
	runtime.RegisterConfig(1)

	runtime.sharedBytes = []byte("script.sh")
	runtime.SetFilePath()
	runtime.sharedBytes = []byte("echo test")

	resultCode := runtime.Format(1)
	if resultCode != uint32(FormatResultNoChange) {
		t.Fatalf("expected no-change result, got %d", resultCode)
	}
	if !handler.lastTokenCancelled {
		t.Fatal("expected cancellation token to read cancellation status from host")
	}
}

func getInt(value any) int {
	switch value := value.(type) {
	case float64:
		return int(value)
	case int:
		return value
	case int64:
		return int(value)
	case uint32:
		return int(value)
	default:
		return 0
	}
}
