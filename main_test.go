package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hrko/dprint-plugin-shfmt/dprint"
)

func TestResolveConfigDefaults(t *testing.T) {
	h := &handler{}

	result := h.ResolveConfig(dprint.ConfigKeyMap{}, dprint.GlobalConfiguration{})

	if result.Config.IndentWidth != 2 {
		t.Fatalf("expected default indent width 2, got %d", result.Config.IndentWidth)
	}
	if result.Config.UseTabs {
		t.Fatal("expected default useTabs to be false")
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %d", len(result.Diagnostics))
	}
}

func TestResolveConfigGlobalOverrideAndDiagnostics(t *testing.T) {
	h := &handler{}

	result := h.ResolveConfig(
		dprint.ConfigKeyMap{
			"indentWidth":  float64(4),
			"useTabs":      false,
			"keepPadding":  "invalid",
			"unknownField": true,
		},
		dprint.GlobalConfiguration{
			"indentWidth": float64(8),
			"useTabs":     true,
		},
	)

	if result.Config.IndentWidth != 8 {
		t.Fatalf("expected global indent width to override plugin value, got %d", result.Config.IndentWidth)
	}
	if !result.Config.UseTabs {
		t.Fatal("expected global useTabs to override plugin value")
	}

	if len(result.Diagnostics) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(result.Diagnostics))
	}
}

func TestResolveConfigCoercesFlexibleValueTypes(t *testing.T) {
	h := &handler{}

	result := h.ResolveConfig(
		dprint.ConfigKeyMap{
			"indentWidth":      []byte("4"),
			"useTabs":          "false",
			"binaryNextLine":   []byte("true"),
			"switchCaseIndent": json.Number("1"),
			"spaceRedirects":   float64(0),
		},
		dprint.GlobalConfiguration{
			"indentWidth": json.Number("8"),
			"useTabs":     []byte("1"),
		},
	)

	if result.Config.IndentWidth != 8 {
		t.Fatalf("expected coerced global indent width 8, got %d", result.Config.IndentWidth)
	}
	if !result.Config.UseTabs {
		t.Fatal("expected coerced global useTabs to be true")
	}
	if !result.Config.BinaryNextLine {
		t.Fatal("expected binaryNextLine to be true")
	}
	if !result.Config.SwitchCaseIndent {
		t.Fatal("expected switchCaseIndent to be true")
	}
	if result.Config.SpaceRedirects {
		t.Fatal("expected spaceRedirects to be false")
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %d", len(result.Diagnostics))
	}
}

func TestResolveConfigIgnoresNilValues(t *testing.T) {
	h := &handler{}

	result := h.ResolveConfig(
		dprint.ConfigKeyMap{
			"indentWidth": nil,
			"useTabs":     nil,
		},
		dprint.GlobalConfiguration{
			"indentWidth": nil,
			"useTabs":     nil,
		},
	)

	if result.Config.IndentWidth != 2 {
		t.Fatalf("expected fallback indent width 2, got %d", result.Config.IndentWidth)
	}
	if result.Config.UseTabs {
		t.Fatal("expected fallback useTabs false")
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics for nil values, got %d", len(result.Diagnostics))
	}
}

func TestFormatWithShfmt(t *testing.T) {
	h := &handler{}

	result := h.Format(
		dprint.SyncFormatRequest[configuration]{
			FilePath: "sample.sh",
			FileBytes: []byte(
				"if [ \"$1\" = \"ok\" ];then\n echo ok\nfi\n",
			),
			Config: configuration{
				IndentWidth: 2,
				UseTabs:     false,
			},
		},
		nil,
	)

	if result.Code != dprint.FormatResultChange {
		t.Fatalf("expected format result code %d, got %d", dprint.FormatResultChange, result.Code)
	}

	expected := "if [ \"$1\" = \"ok\" ]; then\n  echo ok\nfi\n"
	if string(result.Text) != expected {
		t.Fatalf("unexpected formatted output:\n%s", string(result.Text))
	}
}

func TestFormatDetectsBashShebang(t *testing.T) {
	h := &handler{}

	result := h.Format(
		dprint.SyncFormatRequest[configuration]{
			FilePath: "script.sh",
			FileBytes: []byte(
				"#!/usr/bin/env bash\nif [[ \"$a\" == \"b\" ]];then\necho ok\nfi\n",
			),
			Config: configuration{
				IndentWidth: 2,
				UseTabs:     false,
			},
		},
		nil,
	)

	if result.Code == dprint.FormatResultError {
		t.Fatalf("expected bash shebang to parse, got error: %v", result.Err)
	}
}

func TestFormatReturnsErrorOnParseFailure(t *testing.T) {
	h := &handler{}

	result := h.Format(
		dprint.SyncFormatRequest[configuration]{
			FilePath:  "broken.sh",
			FileBytes: []byte("if [ \"$1\" = \"ok\" ]; then\n"),
			Config: configuration{
				IndentWidth: 2,
				UseTabs:     false,
			},
		},
		nil,
	)

	if result.Code != dprint.FormatResultError {
		t.Fatalf("expected format error code %d, got %d", dprint.FormatResultError, result.Code)
	}
	if result.Err == nil || !strings.Contains(result.Err.Error(), "must end with \"fi\"") {
		t.Fatalf("unexpected error text: %v", result.Err)
	}
}
