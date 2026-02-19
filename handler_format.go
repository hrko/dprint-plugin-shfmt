package main

import (
	"bytes"

	"github.com/hrko/dprint-plugin-shfmt/dprint"
	"mvdan.cc/sh/v3/syntax"
)

func (h *handler) Format(
	request dprint.SyncFormatRequest[configuration],
	_ dprint.HostFormatFunc,
) dprint.FormatResult {
	parser := syntax.NewParser(syntax.Variant(detectVariant(request.FilePath, request.FileBytes)))
	prog, err := parser.Parse(bytes.NewReader(request.FileBytes), request.FilePath)
	if err != nil {
		return dprint.FormatError(err)
	}

	printer := syntax.NewPrinter(
		syntax.Indent(indentSize(request.Config)),
		syntax.BinaryNextLine(request.Config.BinaryNextLine),
		syntax.SwitchCaseIndent(request.Config.SwitchCaseIndent),
		syntax.SpaceRedirects(request.Config.SpaceRedirects),
		syntax.FunctionNextLine(request.Config.FuncNextLine),
		syntax.Minify(request.Config.Minify),
	)

	var buffer bytes.Buffer
	if err := printer.Print(&buffer, prog); err != nil {
		return dprint.FormatError(err)
	}

	formatted := buffer.Bytes()
	if bytes.Equal(request.FileBytes, formatted) {
		return dprint.NoChange()
	}

	return dprint.Change(append([]byte(nil), formatted...))
}

func indentSize(config configuration) uint {
	if config.UseTabs {
		return 0
	}
	return uint(config.IndentWidth)
}
