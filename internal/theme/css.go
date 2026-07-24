package theme

import (
	"fmt"
	"strings"
)

// BuildThemeCSS generates a complete theme CSS string for the given colors.
// It prepends @define-color declarations, then appends the rules from
// templateCSS (with its own @define-color lines stripped to avoid duplication).
// The templateCSS is typically the embedded theme-default.css from the gui package.
func BuildThemeCSS(c Colors, templateCSS string) string {
	defs := fmt.Sprintf(
		"@define-color z13-accent      %s;\n"+
			"@define-color z13-accent-glow alpha(@z13-accent, 0.35);\n"+
			"@define-color z13-bg          %s;\n"+
			"@define-color z13-surface     %s;\n"+
			"@define-color z13-surface-alt %s;\n"+
			"@define-color z13-surface-hi  @z13-surface-alt;\n"+
			"@define-color z13-text        %s;\n"+
			"@define-color z13-text-dim    %s;\n"+
			"@define-color z13-text-faint  @z13-text-dim;\n"+
			"@define-color z13-border      %s;\n"+
			"@define-color z13-border-hi   alpha(@z13-text, 0.12);\n"+
			"@define-color z13-success     #4ade80;\n"+
			"@define-color z13-warning     #fbbf24;\n"+
			"@define-color z13-danger      #ef4444;\n",
		c.Accent, c.Background, c.Surface, c.SurfaceAlt, c.Text, c.TextDim, c.Border,
	)
	return defs + "\n" + StripDefineColors(templateCSS)
}

// StripDefineColors removes all @define-color lines from a CSS string.
func StripDefineColors(css string) string {
	var b strings.Builder
	for _, line := range strings.Split(css, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "@define-color") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
