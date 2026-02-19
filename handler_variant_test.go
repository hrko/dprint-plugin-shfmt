package main

import (
	"testing"

	"mvdan.cc/sh/v3/syntax"
)

func TestVariantFromFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		path        string
		wantVariant syntax.LangVariant
		wantOK      bool
	}{
		{name: "sh extension", path: "script.sh", wantVariant: syntax.LangPOSIX, wantOK: true},
		{name: "bash extension", path: "script.bash", wantVariant: syntax.LangBash, wantOK: true},
		{name: "zsh extension uppercase", path: "script.ZSH", wantVariant: syntax.LangBash, wantOK: true},
		{name: "bats extension", path: "test.bats", wantVariant: syntax.LangBash, wantOK: true},
		{name: "mksh extension", path: "script.mksh", wantVariant: syntax.LangMirBSDKorn, wantOK: true},
		{name: "mksh extension uppercase", path: "script.MKSH", wantVariant: syntax.LangMirBSDKorn, wantOK: true},
		{name: "ksh extension unsupported", path: "script.ksh", wantVariant: syntax.LangBash, wantOK: false},
		{name: "unknown extension", path: "script.foo", wantVariant: syntax.LangBash, wantOK: false},
		{name: "no extension", path: "script", wantVariant: syntax.LangBash, wantOK: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotVariant, gotOK := variantFromFilePath(tc.path)
			if gotOK != tc.wantOK {
				t.Fatalf("ok mismatch: want %v, got %v", tc.wantOK, gotOK)
			}
			if gotVariant != tc.wantVariant {
				t.Fatalf("variant mismatch: want %v, got %v", tc.wantVariant, gotVariant)
			}
		})
	}
}

func TestVariantFromShebang(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		fileBytes   []byte
		wantVariant syntax.LangVariant
		wantOK      bool
	}{
		{name: "no shebang", fileBytes: []byte("echo ok\n"), wantVariant: syntax.LangBash, wantOK: false},
		{name: "empty shebang", fileBytes: []byte("#!\n"), wantVariant: syntax.LangBash, wantOK: false},
		{name: "sh shebang", fileBytes: []byte("#!/bin/sh\n"), wantVariant: syntax.LangPOSIX, wantOK: true},
		{name: "dash shebang", fileBytes: []byte("#!/usr/bin/dash\n"), wantVariant: syntax.LangPOSIX, wantOK: true},
		{name: "ash shebang", fileBytes: []byte("#!/bin/ash\n"), wantVariant: syntax.LangPOSIX, wantOK: true},
		{name: "bash shebang", fileBytes: []byte("#!/bin/bash -e\n"), wantVariant: syntax.LangBash, wantOK: true},
		{name: "zsh shebang", fileBytes: []byte("#!/bin/zsh\n"), wantVariant: syntax.LangBash, wantOK: true},
		{name: "bats shebang", fileBytes: []byte("#!/usr/bin/bats\n"), wantVariant: syntax.LangBash, wantOK: true},
		{name: "mksh shebang", fileBytes: []byte("#!/bin/mksh\n"), wantVariant: syntax.LangMirBSDKorn, wantOK: true},
		{name: "mksh uppercase shebang", fileBytes: []byte("#!/BIN/MKSH\n"), wantVariant: syntax.LangMirBSDKorn, wantOK: true},
		{name: "env mksh shebang", fileBytes: []byte("#!/usr/bin/env mksh\n"), wantVariant: syntax.LangMirBSDKorn, wantOK: true},
		{name: "env with options and mksh", fileBytes: []byte("#!/usr/bin/env -S -i mksh -e\n"), wantVariant: syntax.LangMirBSDKorn, wantOK: true},
		{name: "env with options and bash", fileBytes: []byte("#!/usr/bin/env -S bash -e\n"), wantVariant: syntax.LangBash, wantOK: true},
		{name: "env without interpreter", fileBytes: []byte("#!/usr/bin/env -S -i\n"), wantVariant: syntax.LangBash, wantOK: false},
		{name: "unknown shebang", fileBytes: []byte("#!/bin/fish\n"), wantVariant: syntax.LangBash, wantOK: false},
		{name: "ksh shebang unsupported", fileBytes: []byte("#!/bin/ksh\n"), wantVariant: syntax.LangBash, wantOK: false},
		{name: "shebang without trailing newline", fileBytes: []byte("#!/bin/sh"), wantVariant: syntax.LangPOSIX, wantOK: true},
		{name: "shebang with extra spaces", fileBytes: []byte("#!   /bin/sh   \n"), wantVariant: syntax.LangPOSIX, wantOK: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotVariant, gotOK := variantFromShebang(tc.fileBytes)
			if gotOK != tc.wantOK {
				t.Fatalf("ok mismatch: want %v, got %v", tc.wantOK, gotOK)
			}
			if gotVariant != tc.wantVariant {
				t.Fatalf("variant mismatch: want %v, got %v", tc.wantVariant, gotVariant)
			}
		})
	}
}

func TestDetectVariant(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		filePath    string
		fileBytes   []byte
		wantVariant syntax.LangVariant
	}{
		{
			name:        "shebang takes precedence over extension",
			filePath:    "script.sh",
			fileBytes:   []byte("#!/bin/mksh\nset -e\n"),
			wantVariant: syntax.LangMirBSDKorn,
		},
		{
			name:        "file path fallback when no shebang",
			filePath:    "script.mksh",
			fileBytes:   []byte("echo ok\n"),
			wantVariant: syntax.LangMirBSDKorn,
		},
		{
			name:        "file path fallback when shebang unsupported",
			filePath:    "script.sh",
			fileBytes:   []byte("#!/bin/ksh\necho ok\n"),
			wantVariant: syntax.LangPOSIX,
		},
		{
			name:        "default bash when neither shebang nor extension is recognized",
			filePath:    "script.txt",
			fileBytes:   []byte("#!/bin/fish\necho ok\n"),
			wantVariant: syntax.LangBash,
		},
		{
			name:        "default bash for empty path and bytes",
			filePath:    "",
			fileBytes:   nil,
			wantVariant: syntax.LangBash,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotVariant := detectVariant(tc.filePath, tc.fileBytes)
			if gotVariant != tc.wantVariant {
				t.Fatalf("variant mismatch: want %v, got %v", tc.wantVariant, gotVariant)
			}
		})
	}
}
