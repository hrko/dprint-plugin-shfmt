package main

import (
	"bytes"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

func detectVariant(filePath string, fileBytes []byte) syntax.LangVariant {
	if variant, ok := variantFromShebang(fileBytes); ok {
		return variant
	}
	if variant, ok := variantFromFilePath(filePath); ok {
		return variant
	}
	return syntax.LangBash
}

func variantFromFilePath(filePath string) (syntax.LangVariant, bool) {
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(filePath)), ".")
	switch extension {
	case "sh":
		return syntax.LangPOSIX, true
	case "bash", "zsh", "bats":
		return syntax.LangBash, true
	case "mksh":
		return syntax.LangMirBSDKorn, true
	default:
		return syntax.LangBash, false
	}
}

func variantFromShebang(fileBytes []byte) (syntax.LangVariant, bool) {
	if len(fileBytes) < 2 || fileBytes[0] != '#' || fileBytes[1] != '!' {
		return syntax.LangBash, false
	}

	lineEnd := bytes.IndexByte(fileBytes, '\n')
	if lineEnd == -1 {
		lineEnd = len(fileBytes)
	}

	shebang := strings.TrimSpace(strings.TrimPrefix(string(fileBytes[:lineEnd]), "#!"))
	fields := strings.Fields(shebang)
	if len(fields) == 0 {
		return syntax.LangBash, false
	}

	interpreter := strings.ToLower(filepath.Base(fields[0]))
	if interpreter == "env" {
		for _, field := range fields[1:] {
			if field == "-S" || strings.HasPrefix(field, "-") {
				continue
			}
			interpreter = strings.ToLower(filepath.Base(field))
			break
		}
	}

	switch interpreter {
	case "sh", "dash", "ash":
		return syntax.LangPOSIX, true
	case "bash", "zsh", "bats":
		return syntax.LangBash, true
	case "mksh":
		return syntax.LangMirBSDKorn, true
	default:
		return syntax.LangBash, false
	}
}
