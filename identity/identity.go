package identity

import "os"

const (
	ProductName         = "KokoroBox"
	ServiceName         = "KokoroBoxService"
	ServiceDisplayName  = "KokoroBox Service"
	ServiceDescription  = "KokoroBox privileged service"
	ServiceExecutable   = "kokorobox-service"
	ConfigDirectoryName = "KokoroBox"

	WindowsServicePipe = `\\.\pipe\kokorobox\service`
	UnixServiceSocket  = "/tmp/kokorobox-service.sock"

	ConfigDirectoryEnv       = "KOKOROBOX_CONFIG_DIR"
	LegacyConfigDirectoryEnv = "SPARKLE_CONFIG_DIR"
)

// ConfigDirectoryOverride gives new deployments priority while retaining the
// old environment variable as an upgrade-only compatibility input.
func ConfigDirectoryOverride() string {
	if value := os.Getenv(ConfigDirectoryEnv); value != "" {
		return value
	}
	return os.Getenv(LegacyConfigDirectoryEnv)
}
