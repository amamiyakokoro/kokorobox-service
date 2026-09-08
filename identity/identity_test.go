package identity

import "testing"

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
