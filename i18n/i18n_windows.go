package i18n

import (
	"syscall"
	"unsafe"
)

// Locale returns the locale set for the user. If that has not been set, then it falls back to the locale set for the
// system. If that is also unset, then it return "en_US.UTF-8".
func Locale() string {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetUserDefaultLocaleName")
	buffer := make([]uint16, 128)
	if ret, _, _ := proc.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer))); ret == 0 { //nolint:errcheck // ret is the error code
		proc = kernel32.NewProc("GetSystemDefaultLocaleName")
		if ret, _, _ = proc.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer))); ret == 0 { //nolint:errcheck // ret is the error code
			return "en_US.UTF-8"
		}
	}
	return syscall.UTF16ToString(buffer)
}
