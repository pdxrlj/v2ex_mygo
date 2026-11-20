//go:build windows

// Package secure keeps small secrets at rest for the current Windows user.
package secure

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Protect encrypts data for the Windows account running the app, so another
// account or machine cannot decrypt it.
func Protect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))

	encrypted := make([]byte, out.Size)
	copy(encrypted, unsafe.Slice(out.Data, int(out.Size)))
	return encrypted, nil
}

// Unprotect decrypts data Protect made, for the same Windows account.
func Unprotect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	var description *uint16
	if err := windows.CryptUnprotectData(&in, &description, nil, 0, nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
	if description != nil {
		defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(description))))
	}

	plain := make([]byte, out.Size)
	copy(plain, unsafe.Slice(out.Data, int(out.Size)))
	return plain, nil
}
