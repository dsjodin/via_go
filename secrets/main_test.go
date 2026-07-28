package secrets

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testKey is a valid hex encoded AES-256 key.
const testKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	for _, plaintext := range []string{"VMware1!", "", strings.Repeat("x", 4096), "ünïcøde✓"} {
		encrypted, err := Encrypt(plaintext, testKey)
		if err != nil {
			t.Fatalf("Encrypt(%.20q): %v", plaintext, err)
		}
		if encrypted == plaintext && plaintext != "" {
			t.Errorf("Encrypt(%.20q) returned the plaintext", plaintext)
		}

		got, err := Decrypt(encrypted, testKey)
		if err != nil {
			t.Fatalf("Decrypt: %v", err)
		}
		if got != plaintext {
			t.Errorf("round trip = %.20q, want %.20q", got, plaintext)
		}
	}
}

func TestEncryptUsesFreshNonce(t *testing.T) {
	a, err := Encrypt("VMware1!", testKey)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	b, err := Encrypt("VMware1!", testKey)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if a == b {
		t.Error("encrypting the same plaintext twice produced identical ciphertext; nonce is not fresh")
	}
}

// These inputs all used to panic and take the daemon down.
func TestDecryptRejectsBadInputWithoutPanicking(t *testing.T) {
	valid, err := Encrypt("VMware1!", testKey)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	tests := []struct {
		name       string
		ciphertext string
		key        string
	}{
		{"empty ciphertext", "", testKey},
		{"short ciphertext", "abcd", testKey},
		{"non-hex ciphertext", "zzzz", testKey},
		{"non-hex key", valid, "not-hex"},
		{"wrong key length", valid, "0011"},
		{"wrong key", valid, strings.Repeat("ab", 32)},
		{"truncated to nonce only", valid[:24], testKey},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Decrypt(tc.ciphertext, tc.key)
			if err == nil {
				t.Fatalf("Decrypt = %q, want an error", got)
			}
			if got != "" {
				t.Errorf("Decrypt returned %q alongside an error, want empty", got)
			}
		})
	}
}

func TestEncryptRejectsBadKey(t *testing.T) {
	for _, key := range []string{"", "not-hex", "0011"} {
		if _, err := Encrypt("VMware1!", key); err == nil {
			t.Errorf("Encrypt with key %q = nil error, want an error", key)
		}
	}
}

func TestInitGeneratesAndReusesKey(t *testing.T) {
	t.Chdir(t.TempDir())

	key, err := Init()
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	raw, err := hex.DecodeString(key)
	if err != nil {
		t.Fatalf("Init returned a non-hex key: %v", err)
	}
	if len(raw) != keySize {
		t.Errorf("key is %d bytes, want %d", len(raw), keySize)
	}

	info, err := os.Stat(keyFile)
	if err != nil {
		t.Fatalf("stat %s: %v", keyFile, err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("%s mode = %04o, want 0600", keyFile, perm)
	}

	// A second call must reuse the persisted key, not mint a new one — a new
	// key would make every stored password undecryptable.
	again, err := Init()
	if err != nil {
		t.Fatalf("second Init: %v", err)
	}
	if again != key {
		t.Error("Init generated a new key instead of reusing the persisted one")
	}

	// No temporary files may be left behind in the key directory.
	entries, err := os.ReadDir(keyDir)
	if err != nil {
		t.Fatalf("read %s: %v", keyDir, err)
	}
	for _, e := range entries {
		if filepath.Base(e.Name()) != "secret.key" {
			t.Errorf("unexpected leftover file in %s: %s", keyDir, e.Name())
		}
	}
}
