//go:build wasm

package dprint

//go:wasmimport dprint host_write_buffer
func wasmHostWriteBuffer(pointer uint32)

//go:wasmimport dprint host_format
func wasmHostFormat(
	filePathPtr uint32,
	filePathLen uint32,
	rangeStart uint32,
	rangeEnd uint32,
	overrideConfigPtr uint32,
	overrideConfigLen uint32,
	fileBytesPtr uint32,
	fileBytesLen uint32,
) uint32

//go:wasmimport dprint host_get_formatted_text
func wasmHostGetFormattedText() uint32

//go:wasmimport dprint host_get_error_text
func wasmHostGetErrorText() uint32

//go:wasmimport dprint host_has_cancelled
func wasmHostHasCancelled() uint32

func hostWriteBuffer(pointer uint32) {
	wasmHostWriteBuffer(pointer)
}

func hostFormat(
	filePathPtr uint32,
	filePathLen uint32,
	rangeStart uint32,
	rangeEnd uint32,
	overrideConfigPtr uint32,
	overrideConfigLen uint32,
	fileBytesPtr uint32,
	fileBytesLen uint32,
) uint32 {
	return wasmHostFormat(
		filePathPtr,
		filePathLen,
		rangeStart,
		rangeEnd,
		overrideConfigPtr,
		overrideConfigLen,
		fileBytesPtr,
		fileBytesLen,
	)
}

func hostGetFormattedText() uint32 {
	return wasmHostGetFormattedText()
}

func hostGetErrorText() uint32 {
	return wasmHostGetErrorText()
}

func hostHasCancelled() uint32 {
	return wasmHostHasCancelled()
}
