package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppConfig(t *testing.T) {
	tests := []struct {
		name, data string
		want       AppConfig
	}{
		{"missing", "", AppConfig{Theme: "rog-dark"}},
		{"theme and accent", "theme = \"nord\"\naccent = \"blue\"\n", AppConfig{Theme: "nord", Accent: "blue"}},
		{"comments", "# config file\ntheme = \"gruvbox-dark\" # nice theme\n", AppConfig{Theme: "gruvbox-dark"}},
		{"empty theme", "theme = \"\"\n", AppConfig{Theme: "rog-dark"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", tmp)
			if tt.data != "" {
				dir := filepath.Join(tmp, "z13gui-plus")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(tt.data), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := LoadAppConfig(); got != tt.want {
				t.Errorf("LoadAppConfig() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSaveAppConfig(t *testing.T) {
	for _, want := range []AppConfig{
		{Theme: "catppuccin-mocha", Accent: "sapphire"},
		{Theme: "nord"},
	} {
		t.Run(want.Theme, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", tmp)
			SaveAppConfig(want)
			if got := LoadAppConfig(); got != want {
				t.Errorf("round trip = %+v, want %+v", got, want)
			}
			info, err := os.Stat(filepath.Join(tmp, "z13gui-plus"))
			if err != nil || !info.IsDir() {
				t.Fatalf("config directory was not created: %v", err)
			}
		})
	}
}

func TestXDGConfigHome(t *testing.T) {
	t.Run("environment", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "/custom/config")
		if got := XDGConfigHome(); got != "/custom/config" {
			t.Errorf("XDGConfigHome() = %q, want /custom/config", got)
		}
	})
	t.Run("fallback", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		want, err := os.UserConfigDir()
		if err != nil {
			t.Fatal(err)
		}
		if got := XDGConfigHome(); got != want {
			t.Errorf("XDGConfigHome() = %q, want %q", got, want)
		}
	})
}

func TestMigrateLegacyConfigCopiesWithoutOverwrite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(LegacyConfigDir(), "themes"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(LegacyConfigDir(), "config.toml"), []byte("theme = \"nord\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(LegacyConfigDir(), "themes", "custom.css"), []byte("window {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := MigrateLegacyConfig(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(ConfigDir(), "themes", "custom.css"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "window {}\n" {
		t.Fatalf("migrated custom theme = %q", got)
	}
	if _, statErr := os.Stat(filepath.Join(LegacyConfigDir(), "config.toml")); statErr != nil {
		t.Fatalf("legacy config was removed: %v", statErr)
	}

	if writeErr := os.WriteFile(filepath.Join(LegacyConfigDir(), "config.toml"), []byte("theme = \"rog-dark\"\n"), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	if migrateErr := MigrateLegacyConfig(); migrateErr == nil {
		t.Fatal("MigrateLegacyConfig overwrote existing Plus config")
	}
	got, err = os.ReadFile(filepath.Join(ConfigDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "theme = \"nord\"\n" {
		t.Fatalf("config changed after refused migration: %q", got)
	}
}

func TestMigrateLegacyConfigRequiresSource(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := MigrateLegacyConfig(); err == nil {
		t.Fatal("MigrateLegacyConfig accepted a missing source")
	}
	if _, err := os.Stat(ConfigDir()); !os.IsNotExist(err) {
		t.Fatalf("Plus config exists after failed migration: %v", err)
	}
}
