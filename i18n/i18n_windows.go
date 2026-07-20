package i18n

import (
	"syscall"
	"unsafe"
)

// sysLocale returns the raw OS locale string (e.g. "en-US") for the user,
// falling back to the system default. It may be empty.
func sysLocale() string {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetUserDefaultLocaleName")
	buffer := make([]uint16, 128)
	if ret, _, _ := proc.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer))); ret == 0 { //nolint:errcheck // ret is the error code
		proc = kernel32.NewProc("GetSystemDefaultLocaleName")
		if ret, _, _ = proc.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer))); ret == 0 { //nolint:errcheck // ret is the error code
			return ""
		}
	}
	return syscall.UTF16ToString(buffer)
}
