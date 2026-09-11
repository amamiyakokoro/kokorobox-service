package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func testBootstrapPublicKey(t *testing.T) string {
	t.Helper()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func testBootstrapKeyManager(t *testing.T) *KeyManager {
	t.Helper()
	dir := t.TempDir()
	return &KeyManager{
		keyRingPath:   filepath.Join(dir, "public_keys.json"),
		principalPath: filepath.Join(dir, "authorized_principal.json"),
	}
}

func TestBootstrapPublicKeyForUIDInitializesOnce(t *testing.T) {
	km := testBootstrapKeyManager(t)
	if err := km.BootstrapPublicKeyForUID(testBootstrapPublicKey(t), 501); err != nil {
		t.Fatal(err)
	}
	if !km.IsInitialized() || !km.HasAuthorizedPrincipal() {
		t.Fatal("bootstrap did not initialize authentication state")
	}
	if uid, ok := km.GetAuthorizedUID(); !ok || uid != 501 {
		t.Fatalf("unexpected authorized UID: %d, %v", uid, ok)
	}
	for _, path := range []string{km.keyRingPath, km.principalPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode is %o, want 600", path, info.Mode().Perm())
		}
	}
	if err := km.BootstrapPublicKeyForUID(testBootstrapPublicKey(t), 502); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("second bootstrap error = %v, want ErrAlreadyInitialized", err)
	}
}

func TestBootstrapPublicKeyForUIDNeverOverwritesExistingState(t *testing.T) {
	km := testBootstrapKeyManager(t)
	if err := os.WriteFile(km.keyRingPath, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := km.BootstrapPublicKeyForUID(testBootstrapPublicKey(t), 501); err == nil {
		t.Fatal("bootstrap unexpectedly replaced existing state")
	}
	data, err := os.ReadFile(km.keyRingPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "existing" {
		t.Fatalf("existing state was modified: %q", data)
	}
}
