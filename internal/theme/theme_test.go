package theme

import "testing"

func TestBuiltinsNotEmpty(t *testing.T) {
	if len(Builtins) == 0 {
		t.Fatal("Builtins slice is empty")
	}
}

func TestBuiltinsUniqueIDs(t *testing.T) {
	seen := make(map[string]bool)
	for _, b := range Builtins {
		if seen[b.ID] {
			t.Errorf("duplicate builtin theme ID: %q", b.ID)
		}
		seen[b.ID] = true
	}
}

func TestBuiltinsAllColorsSet(t *testing.T) {
	for _, b := range Builtins {
		c := b.Colors
		if c.Accent == "" || c.Background == "" || c.Surface == "" ||
			c.SurfaceAlt == "" || c.Text == "" || c.TextDim == "" || c.Border == "" {
			t.Errorf("theme %q has empty color fields", b.ID)
		}
	}
}

func TestBuiltinByID(t *testing.T) {
	for _, tt := range []struct {
		id         string
		wantAccent string
		wantOK     bool
	}{
		{"rog-dark", "#cc0000", true},
		{"nonexistent-theme", "", false},
	} {
		colors, ok := BuiltinByID(tt.id)
		if ok != tt.wantOK || colors.Accent != tt.wantAccent {
			t.Errorf("BuiltinByID(%q) = (%+v, %v), want accent %q, ok %v", tt.id, colors, ok, tt.wantAccent, tt.wantOK)
		}
	}
}

func TestBuiltinByID_AllThemes(t *testing.T) {
	for _, b := range Builtins {
		c, ok := BuiltinByID(b.ID)
		if !ok {
			t.Errorf("BuiltinByID(%q) returned false", b.ID)
			continue
		}
		if c != b.Colors {
			t.Errorf("BuiltinByID(%q) returned mismatched colors", b.ID)
		}
	}
}

func TestBuiltinAccentHex(t *testing.T) {
	for _, tt := range []struct {
		theme, accent, want string
		wantOK              bool
	}{
		{"catppuccin-mocha", "blue", "#89b4fa", true},
		{"catppuccin-mocha", "nonexistent", "", false},
		{"nonexistent-theme", "blue", "", false},
		{"rog-dark", "blue", "", false},
	} {
		hex, ok := BuiltinAccentHex(tt.theme, tt.accent)
		if hex != tt.want || ok != tt.wantOK {
			t.Errorf("BuiltinAccentHex(%q, %q) = (%q, %v), want (%q, %v)", tt.theme, tt.accent, hex, ok, tt.want, tt.wantOK)
		}
	}
}

func TestBuiltinAccentHex_AllAccents(t *testing.T) {
	for _, b := range Builtins {
		for _, a := range b.Accents {
			hex, ok := BuiltinAccentHex(b.ID, a.ID)
			if !ok {
				t.Errorf("BuiltinAccentHex(%q, %q) returned false", b.ID, a.ID)
				continue
			}
			if hex != a.Hex {
				t.Errorf("BuiltinAccentHex(%q, %q) = %q, want %q", b.ID, a.ID, hex, a.Hex)
			}
		}
	}
}

func TestDefaultColorsValid(t *testing.T) {
	c := DefaultColors
	for _, pair := range []struct {
		name, val string
	}{
		{"Accent", c.Accent},
		{"Background", c.Background},
		{"Surface", c.Surface},
		{"SurfaceAlt", c.SurfaceAlt},
		{"Text", c.Text},
		{"TextDim", c.TextDim},
		{"Border", c.Border},
	} {
		if !IsHexColor(pair.val) {
			t.Errorf("DefaultColors.%s = %q is not a valid hex color", pair.name, pair.val)
		}
	}
}
