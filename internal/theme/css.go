package theme

import (
	"fmt"
	"math"
	"strings"
)

// dimContrastFloor is WCAG AA for small text. Every section label, caption and metric
// label in the drawer is dim, and measured against the card background several stock
// palettes fall short of it (ROG Dark 4.05, Gruvbox Dark 4.17, Catppuccin Frappe 4.26,
// Tokyo Night 2.35, Solarized Light 2.58, Rose Pine Dawn 2.48, Everforest Light 2.83,
// Catppuccin Latte 3.25, Gruvbox Light 3.55), so the value is clamped rather than
// trusted. Cards paint surface-alt (surface-hi is defined as surface-alt), which is
// the harsher of the two backgrounds in both light and dark themes.
const dimContrastFloor = 4.5

// ensureContrast returns color unchanged when it already reaches min contrast against
// bg, otherwise the first blend of color toward white (dark bg) or black (light bg)
// that does. Returns color untouched if nothing qualifies or the input is unparseable,
// so a malformed theme can never blank out the drawer's text.
func ensureContrast(color, bg string, floor float64) string {
	if contrastRatio(color, bg) >= floor {
		return color
	}
	target := "#ffffff"
	if relativeLuminance(bg) > 0.5 {
		target = "#000000"
	}
	for i := 1; i <= 40; i++ {
		blend := mixHex(color, target, float64(i)/40)
		if contrastRatio(blend, bg) >= floor {
			return blend
		}
	}
	return color
}

func contrastRatio(a, b string) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func relativeLuminance(hex string) float64 {
	r, g, b, ok := parseHex(hex)
	if !ok {
		return 0
	}
	lin := func(c float64) float64 {
		if c <= 0.04045 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

func mixHex(a, b string, t float64) string {
	a1, a2, a3, ok1 := parseHex(a)
	b1, b2, b3, ok2 := parseHex(b)
	if !ok1 || !ok2 {
		return a
	}
	mix := func(x, y float64) int {
		return int((x+(y-x)*t)*255 + 0.5)
	}
	return fmt.Sprintf("#%02x%02x%02x", mix(a1, b1), mix(a2, b2), mix(a3, b3))
}

func parseHex(hex string) (r, g, b float64, ok bool) {
	h := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(h) != 6 {
		return 0, 0, 0, false
	}
	var v [3]float64
	for i := range v {
		n := 0
		for _, c := range h[i*2 : i*2+2] {
			n <<= 4
			switch {
			case c >= '0' && c <= '9':
				n += int(c - '0')
			case c >= 'a' && c <= 'f':
				n += int(c-'a') + 10
			case c >= 'A' && c <= 'F':
				n += int(c-'A') + 10
			default:
				return 0, 0, 0, false
			}
		}
		v[i] = float64(n) / 255
	}
	return v[0], v[1], v[2], true
}

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
		c.Accent, c.Background, c.Surface, c.SurfaceAlt, c.Text,
		ensureContrast(c.TextDim, c.SurfaceAlt, dimContrastFloor), c.Border,
	)

	var template strings.Builder
	for _, line := range strings.Split(templateCSS, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "@define-color") {
			continue
		}
		template.WriteString(line)
		template.WriteByte('\n')
	}
	return defs + "\n" + template.String()
}
