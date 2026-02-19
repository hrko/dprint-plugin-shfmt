package dprint

import "unsafe"

func (r *Runtime[T]) takeSharedBytes() []byte {
	bytes := r.sharedBytes
	r.sharedBytes = nil
	return bytes
}

func (r *Runtime[T]) setSharedBytes(bytes []byte) uint32 {
	r.sharedBytes = bytes
	return uint32(len(bytes))
}

func bytesPtr(bytes []byte) uint32 {
	if len(bytes) == 0 {
		return 0
	}
	return uint32(uintptr(unsafe.Pointer(&bytes[0])))
}
