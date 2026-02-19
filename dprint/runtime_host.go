package dprint

import (
	"encoding/json"
	"fmt"
)

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

type hostCancellationToken struct{}

func (hostCancellationToken) IsCancelled() bool {
	return hostHasCancelled() == 1
}
