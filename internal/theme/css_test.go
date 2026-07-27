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
		"#ff0000", "#111111", "#222222", "#333333", "#eeeeee", "#999999", "#555555",
		".drawer", ".bottom-bar",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("output missing %q", want)
		}
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
