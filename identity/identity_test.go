package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDirectoryOverridePrefersKokoroBoxEnvironment(t *testing.T) {
	t.Setenv(LegacyConfigDirectoryEnv, "/legacy")
	t.Setenv(ConfigDirectoryEnv, "/kokorobox")

	if actual := ConfigDirectoryOverride(); actual != "/kokorobox" {
		t.Fatalf("ConfigDirectoryOverride() = %q, want %q", actual, "/kokorobox")
	}
}

func TestConfigDirectoryOverrideAcceptsLegacyEnvironment(t *testing.T) {
	t.Setenv(ConfigDirectoryEnv, "")
	t.Setenv(LegacyConfigDirectoryEnv, "/legacy")

	if actual := ConfigDirectoryOverride(); actual != "/legacy" {
		t.Fatalf("ConfigDirectoryOverride() = %q, want %q", actual, "/legacy")
	}
}

func TestDataDirectoryMigratesLegacyDirectoryWhenCurrentIsMissing(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "sparkle")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "state"), []byte("preserved"), 0o600); err != nil {
		t.Fatal(err)
	}

	current, err := DataDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	if current != filepath.Join(root, ConfigDirectoryName) {
		t.Fatalf("DataDirectory() = %q", current)
	}
	if _, err := os.Stat(filepath.Join(current, "state")); err != nil {
		t.Fatalf("migrated state missing: %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy directory still exists: %v", err)
	}
}

func TestDataDirectoryDoesNotOverwriteCurrentDirectory(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, ConfigDirectoryName)
	legacy := filepath.Join(root, "sparkle")
	for _, dir := range []string{current, legacy} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	actual, err := DataDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	if actual != current {
		t.Fatalf("DataDirectory() = %q, want %q", actual, current)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("legacy directory should be preserved: %v", err)
	}
}
