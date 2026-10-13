package theme

import (
	"strings"
	"testing"
)

func TestBuildThemeCSS(t *testing.T) {
	colors := Colors{
		Accent: "#ff0000", Background: "#111111", Surface: "#222222",
		SurfaceAlt: "#333333", Text: "#eeeeee", TextDim: "#999999", Border: "#555555",
	}
	template := "  @define-color z13-accent #old;\n.drawer { background: @z13-bg; }\n.bottom-bar { margin: 4px; }\n"
	css := BuildThemeCSS(colors, template)

	for _, want := range []string{
		"@define-color z13-accent ", "@define-color z13-bg ",
		"@define-color z13-surface ", "@define-color z13-surface-alt ",
		"@define-color z13-text ", "@define-color z13-text-dim ",
		"@define-color z13-border ", "@define-color z13-accent-glow ",
		"@define-color z13-surface-hi ", "@define-color z13-text-faint ",
		"@define-color z13-border-hi ", "@define-color z13-success ",
		"@define-color z13-warning ", "@define-color z13-danger ",
		"#ff0000", "#111111", "#222222", "#333333", "#eeeeee", "#555555",
		".drawer", ".bottom-bar",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("output missing %q", want)
		}
	}
	// text-dim is emitted clamped to the contrast floor, so it is deliberately not the
	// raw palette value (#999999 measures 4.05:1 on the card background).
	if want := "z13-text-dim    " + ensureContrast("#999999", "#333333", dimContrastFloor); !strings.Contains(css, want) {
		t.Errorf("output missing %q", want)
	}
	if strings.Contains(css, "#old") {
		t.Error("template @define-color was not stripped")
	}
}

func TestBuildThemeCSSStripsAllTemplateDefinitions(t *testing.T) {
	template := "@define-color z13-accent #old;\n.foo { color: red; }\n  @define-color z13-bg #old;\n.bar { }\n"
	css := BuildThemeCSS(DefaultColors, template)
	if strings.Contains(css, "#old") {
		t.Error("template @define-color lines were not stripped")
	}
	for _, rule := range []string{".foo", ".bar"} {
		if !strings.Contains(css, rule) {
			t.Errorf("template rule %s was incorrectly stripped", rule)
		}
	}
}

func TestBuildThemeCSSEmptyTemplate(t *testing.T) {
	if css := BuildThemeCSS(DefaultColors, ""); !strings.Contains(css, "@define-color z13-accent") {
		t.Error("empty template output is missing color definitions")
	}
}

// Every section label, caption and metric label in the drawer is dim text, so the dim
// token has to clear WCAG AA on the card background for every palette a user can pick
// without editing a file. Several stock palettes measured below it before the clamp.
func TestDimTextClearsContrastFloorEveryBuiltin(t *testing.T) {
	check := func(name string, c Colors) {
		dim := ensureContrast(c.TextDim, c.SurfaceAlt, dimContrastFloor)
		if r := contrastRatio(dim, c.SurfaceAlt); r < dimContrastFloor {
			t.Errorf("%s: dim %s on card %s is %.2f:1, want >= %.1f:1", name, dim, c.SurfaceAlt, r, dimContrastFloor)
		}
	}
	check("default", DefaultColors)
	for _, b := range Builtins {
		check(b.Name, b.Colors)
	}
}

// A palette that already complies must keep its own colour — the clamp is a floor, not
// a re-tint, otherwise editing a theme would visibly change nothing it was set to.
func TestEnsureContrastLeavesCompliantColourAlone(t *testing.T) {
	if got := ensureContrast("#e0e0e0", "#1a1a1a", 4.5); got != "#e0e0e0" {
		t.Errorf("compliant colour changed to %s", got)
	}
	if got := ensureContrast("#notahex", "#2a2a2a", 4.5); got != "#notahex" {
		t.Errorf("malformed colour should pass through, got %s", got)
	}
}
