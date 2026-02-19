package dprint

import (
	"encoding/json"
	"fmt"
)

type hostFormatRequest struct {
	filePath       string
	rangeStart     uint32
	rangeEnd       uint32
	overrideConfig []byte
	fileBytes      []byte
}

type hostBridge interface {
	writeBuffer(pointer uint32)
	format(request hostFormatRequest) uint32
	readFormattedText(readBytesFromHost func(length uint32) []byte) []byte
	readErrorText(readBytesFromHost func(length uint32) []byte) string
	hasCancelled() bool
}

type wasmHostBridge struct{}

func (wasmHostBridge) writeBuffer(pointer uint32) {
	hostWriteBuffer(pointer)
}

func (wasmHostBridge) format(request hostFormatRequest) uint32 {
	filePathBytes := []byte(request.filePath)
	return hostFormat(
		bytesPtr(filePathBytes),
		uint32(len(filePathBytes)),
		request.rangeStart,
		request.rangeEnd,
		bytesPtr(request.overrideConfig),
		uint32(len(request.overrideConfig)),
		bytesPtr(request.fileBytes),
		uint32(len(request.fileBytes)),
	)
}

func (wasmHostBridge) readFormattedText(readBytesFromHost func(length uint32) []byte) []byte {
	return readBytesFromHost(hostGetFormattedText())
}

func (wasmHostBridge) readErrorText(readBytesFromHost func(length uint32) []byte) string {
	return string(readBytesFromHost(hostGetErrorText()))
}

func (wasmHostBridge) hasCancelled() bool {
	return hostHasCancelled() == 1
}

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

	resultCode := r.host.format(hostFormatRequest{
		filePath:       request.FilePath,
		rangeStart:     startRange,
		rangeEnd:       endRange,
		overrideConfig: overrideConfigBytes,
		fileBytes:      request.FileBytes,
	})

	switch resultCode {
	case uint32(FormatResultNoChange):
		return NoChange()
	case uint32(FormatResultChange):
		return Change(r.host.readFormattedText(r.readBytesFromHost))
	case uint32(FormatResultError):
		return FormatError(fmt.Errorf("%s", r.host.readErrorText(r.readBytesFromHost)))
	default:
		panic(fmt.Sprintf("unknown host format value: %d", resultCode))
	}
}

func (r *Runtime[T]) readBytesFromHost(length uint32) []byte {
	ptr := r.ClearSharedBytes(length)
	r.host.writeBuffer(ptr)
	return r.takeSharedBytes()
}

type hostCancellationToken struct {
	host hostBridge
}

func (t hostCancellationToken) IsCancelled() bool {
	if t.host == nil {
		return false
	}
	return t.host.hasCancelled()
}
