//go:build !windows

package i18n

import "os"

// sysLocale returns the raw OS locale string (e.g. "en_US.UTF-8") from
// LC_ALL, falling back to LANG. It may be empty.
func sysLocale() string {
	if locale := os.Getenv("LC_ALL"); locale != "" {
		return locale
	}
	return os.Getenv("LANG")
}
