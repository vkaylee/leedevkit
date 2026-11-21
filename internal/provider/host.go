package provider

import (
	"os"
	"path/filepath"
)

func HostConfigPath(environ map[string]string, home string) string {
	if root := environ["XDG_CONFIG_HOME"]; root != "" {
		return filepath.Join(root, "leedevkit", "config.toml")
	}
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return filepath.Join(home, ".config", "leedevkit", "config.toml")
}
