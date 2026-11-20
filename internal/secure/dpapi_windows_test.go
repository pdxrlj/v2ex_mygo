//go:build windows

package secure

import "testing"

// TestRoundTrip checks that DPAPI gives the data back to the same user and
// does not store it as plain text.
func TestRoundTrip(t *testing.T) {
	plain := []byte("A2=secret-cookie-value")

	encrypted, err := Protect(plain)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if len(encrypted) == 0 {
		t.Fatal("Protect returned no data")
	}
	if string(encrypted) == string(plain) {
		t.Fatal("Protect returned the plain text")
	}

	decrypted, err := Unprotect(encrypted)
	if err != nil {
		t.Fatalf("Unprotect: %v", err)
	}
	if string(decrypted) != string(plain) {
		t.Errorf("Unprotect() = %q, want %q", decrypted, plain)
	}
}
