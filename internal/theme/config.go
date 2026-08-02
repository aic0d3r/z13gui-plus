package theme

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// AppConfig holds app-level preferences persisted to config.toml.
type AppConfig struct {
	Theme  string // built-in theme ID; empty = use default
	Accent string // accent ID within the theme; "" = use theme default
}

// LoadAppConfig reads config.toml from the z13gui-plus config directory.
// Returns a default config (theme "rog-dark") if the file doesn't exist or can't be parsed.
func LoadAppConfig() AppConfig {
	path := filepath.Join(ConfigDir(), "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return AppConfig{Theme: "rog-dark"}
	}
	cfg := AppConfig{Theme: "rog-dark"}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.Index(line, " #"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		switch k {
		case "theme":
			if v != "" {
				cfg.Theme = v
			}
		case "accent":
			cfg.Accent = v
		}
	}
	return cfg
}

// SaveAppConfig writes the app config to the z13gui-plus config directory.
func SaveAppConfig(cfg AppConfig) {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Warn("failed to create config dir", "path", dir, "err", err)
		return
	}
	content := "# Z13GUI+ app configuration\ntheme = \"" + cfg.Theme + "\"\n"
	if cfg.Accent != "" {
		content += "accent = \"" + cfg.Accent + "\"\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(content), 0o644); err != nil {
		slog.Warn("failed to write config", "err", err)
	}
}

// XDGConfigHome returns the current user's configuration directory.
func XDGConfigHome() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		slog.Warn("failed to get config directory", "err", err)
		return "/tmp/.config"
	}
	return dir
}

// ConfigDir returns the z13gui-plus configuration directory.
func ConfigDir() string {
	return filepath.Join(XDGConfigHome(), "z13gui-plus")
}

// LegacyConfigDir returns the configuration directory used before v2.
func LegacyConfigDir() string {
	return filepath.Join(XDGConfigHome(), "z13gui")
}

// MigrateLegacyConfig copies legacy configuration without overwriting Plus config.
func MigrateLegacyConfig() error {
	source := LegacyConfigDir()
	destination := ConfigDir()

	info, err := os.Stat(source)
	if os.IsNotExist(err) {
		return fmt.Errorf("legacy config does not exist: %s", source)
	}
	if err != nil {
		return fmt.Errorf("read legacy config: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("legacy config is not a directory: %s", source)
	}
	if mkdirAllErr := os.MkdirAll(filepath.Dir(destination), 0o700); mkdirAllErr != nil {
		return fmt.Errorf("create config parent: %w", mkdirAllErr)
	}
	if mkdirErr := os.Mkdir(destination, 0o700); os.IsExist(mkdirErr) {
		return fmt.Errorf("z13gui-plus config already exists: %s", destination)
	} else if mkdirErr != nil {
		return fmt.Errorf("create z13gui-plus config: %w", mkdirErr)
	}
	if copyErr := os.CopyFS(destination, os.DirFS(source)); copyErr != nil {
		_ = os.RemoveAll(destination)
		return fmt.Errorf("copy legacy config: %w", copyErr)
	}
	return nil
}
