//go:build !wasm

// Package dprint provides minimal types and runtime glue for dprint Wasm plugins.
package dprint

func hostWriteBuffer(_ uint32) {
	panic("dprint host imports are only available in wasm")
}

func hostFormat(
	_ uint32,
	_ uint32,
	_ uint32,
	_ uint32,
	_ uint32,
	_ uint32,
	_ uint32,
	_ uint32,
) uint32 {
	panic("dprint host imports are only available in wasm")
}

func hostGetFormattedText() uint32 {
	panic("dprint host imports are only available in wasm")
}

func hostGetErrorText() uint32 {
	panic("dprint host imports are only available in wasm")
}

func hostHasCancelled() uint32 {
	return 0
}
