//go:build !windows

// Package secure keeps small secrets at rest for the current Windows user.
// Other platforms, which this app does not target, keep them as they are.
package secure

// Protect returns data unchanged where DPAPI is unavailable.
func Protect(data []byte) ([]byte, error) {
	return append([]byte(nil), data...), nil
}

// Unprotect returns data unchanged where DPAPI is unavailable.
func Unprotect(data []byte) ([]byte, error) {
	return append([]byte(nil), data...), nil
}
