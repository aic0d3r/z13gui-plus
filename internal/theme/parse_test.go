package theme

import (
	"reflect"
	"testing"
)

func TestIsHexColor(t *testing.T) {
	tests := map[string]bool{
		"#abc": true, "#ABC": true, "#aabbcc": true, "#AABBCC": true,
		"#aabbccdd": true, "#AABBCCDD": true, "#123": true,
		"#112233": true, "#11223344": true, "": false, "abc": false,
		"#": false, "#ab": false, "#abcd": false, "#abcde": false,
		"#abcdefg": false, "#gggggg": false, "#zzzzzz": false,
		"#abcdef ": false, " #abcdef": false, "#12345": false,
	}
	for input, want := range tests {
		if got := IsHexColor(input); got != want {
			t.Errorf("IsHexColor(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestParseThemeTOMLFullColors(t *testing.T) {
	full := Colors{
		Accent: "#ff0000", Background: "#111111", Surface: "#222222",
		SurfaceAlt: "#333333", Text: "#eeeeee", TextDim: "#999999", Border: "#555555",
	}
	accentOnly := DefaultColors
	accentOnly.Accent = "#ff0000"
	aabbcc := DefaultColors
	aabbcc.Accent = "#aabbcc"
	commented := aabbcc
	commented.Background = "#112233"

	tests := []struct {
		name, input string
		want        Colors
	}{
		{"full", `accent = "#ff0000"
background = "#111111"
surface = "#222222"
surface_alt = "#333333"
text = "#eeeeee"
text_dim = "#999999"
border = "#555555"`, full},
		{"missing keys", `accent = "#ff0000"`, accentOnly},
		{"invalid hex", `accent = "not-a-color"`, DefaultColors},
		{"comments and blanks", `
# comment
accent = "#aabbcc"

# another comment
background = "#112233"
`, commented},
		{"inline comment", `accent = "#aabbcc" # accent color`, aabbcc},
		{"single quoted", `accent = '#aabbcc'`, aabbcc},
		{"unknown keys", "accent = \"#ff0000\"\nunknown_key = \"#00ff00\"\nanother_thing = \"hello\"", accentOnly},
		{"empty", "", DefaultColors},
		{"no equals", "this is not valid toml\n", DefaultColors},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := ParseThemeTOMLFull([]byte(tt.input))
			if got != tt.want {
				t.Errorf("colors = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseThemeTOMLFullAccents(t *testing.T) {
	tests := []struct {
		name, input string
		want        []Accent
	}{
		{"none", `accent = "#ff0000"`, nil},
		{"variants", `[accents]
blue = "#5294e2"
teal = "#2eb398"
purple = "#9b59b6"`, []Accent{
			{ID: "blue", Name: "Blue", Hex: "#5294e2"},
			{ID: "teal", Name: "Teal", Hex: "#2eb398"},
			{ID: "purple", Name: "Purple", Hex: "#9b59b6"},
		}},
		{"invalid hex", `[accents]
good = "#aabbcc"
bad = "not-a-color"
also_good = "#112233"`, []Accent{
			{ID: "good", Name: "Good", Hex: "#aabbcc"},
			{ID: "also_good", Name: "Also_good", Hex: "#112233"},
		}},
		{"file order", `[accents]
cherry = "#ff0000"
apple = "#00ff00"
banana = "#0000ff"`, []Accent{
			{ID: "cherry", Name: "Cherry", Hex: "#ff0000"},
			{ID: "apple", Name: "Apple", Hex: "#00ff00"},
			{ID: "banana", Name: "Banana", Hex: "#0000ff"},
		}},
		{"next section", `[accents]
blue = "#0000ff"
[other]
something = "#00ff00"`, []Accent{{ID: "blue", Name: "Blue", Hex: "#0000ff"}}},
		{"comments", `[accents]
# comment
blue = "#0000ff"
# another comment
red = "#ff0000"`, []Accent{
			{ID: "blue", Name: "Blue", Hex: "#0000ff"},
			{ID: "red", Name: "Red", Hex: "#ff0000"},
		}},
		{"title case", `[accents]
deep_blue = "#0000ff"
RED = "#ff0000"`, []Accent{
			{ID: "deep_blue", Name: "Deep_blue", Hex: "#0000ff"},
			{ID: "RED", Name: "RED", Hex: "#ff0000"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := ParseThemeTOMLFull([]byte(tt.input))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("accents = %+v, want %+v", got, tt.want)
			}
		})
	}
}
