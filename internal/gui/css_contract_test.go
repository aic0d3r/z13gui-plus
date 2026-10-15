package gui

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Touch-floor contract for the drawer.
//
// The drawer is operated by finger (and by gamepad), so every tappable row and button
// declares a min-height. 40px is the floor this drawer already uses everywhere; the
// platform guidance is 44-48px, and 40 is what still fits two-column grids in a 320px
// drawer. The disclosure header is the one place held to 44px.
//
// Gamescope scales the whole drawer by outputWidth/1707 (~0.75 on a 1280px display),
// so a value below the floor is already marginal before any scaling. Do not lower one of
// these numbers to make a layout fit - restructure the row instead.

const touchFloor = 40

// selectors that carry a tap: Button / ToggleButton / CheckButton, or a row with a tap
// gesture (see addTouchActivate in controls.go).
var touchFloors = map[string]int{
	".drawer .action-btn":                 touchFloor,
	".drawer .outline-btn":                touchFloor,
	".drawer .subtle-btn":                 touchFloor,
	".drawer .btn-group button":           touchFloor,
	".drawer .main-tab-row button":        touchFloor,
	".drawer checkbutton":                 touchFloor,
	".drawer .mode-grid.btn-group button": touchFloor,
	".drawer .settings-row":               touchFloor,
	".custom-actions button":              touchFloor,
	".advanced-check":                     touchFloor,
	".tab-btn":                            touchFloor,
	".color-swatch":                       touchFloor,
	".color-preset":                       touchFloor,
	".drawer .view-back-btn":              touchFloor,
	".drawer .section-collapse":           44,
}

// Tappable but deliberately under the floor, recorded so it cannot shrink quietly.
// .tablet-chip sits in the title row; 36px is a comfortable touch target without
// growing that row much. shortcut: recorded floor, revisit if the header layout changes.
var belowFloorButKnown = map[string]int{
	".drawer .tablet-chip": 36,
}

var (
	cssCommentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssBlockRe   = regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`)
	minHeightRe  = regexp.MustCompile(`min-height:\s*(\d+)px`)
)

// minHeightBySelector maps each selector of a flat rule to its declared min-height in px.
// layout.css is flat today (no @media / @keyframes); a nested block would need a real parser.
func minHeightBySelector(css string) map[string]int {
	out := map[string]int{}
	for _, rule := range cssBlockRe.FindAllStringSubmatch(cssCommentRe.ReplaceAllString(css, ""), -1) {
		mh := minHeightRe.FindStringSubmatch(rule[2])
		if mh == nil {
			continue
		}
		n, err := strconv.Atoi(mh[1])
		if err != nil {
			continue
		}
		for _, sel := range strings.Split(rule[1], ",") {
			if sel = strings.TrimSpace(sel); sel != "" {
				out[sel] = n
			}
		}
	}
	return out
}

func TestTouchTargetsKeepTheFloor(t *testing.T) {
	declared := minHeightBySelector(layoutCSS)

	for selector, floor := range touchFloors {
		got, ok := declared[selector]
		if !ok {
			t.Errorf("%s: no min-height declared - a tap target must declare one", selector)
			continue
		}
		if got < floor {
			t.Errorf("%s: min-height %dpx is below the %dpx touch floor", selector, got, floor)
		}
	}

	for selector, recorded := range belowFloorButKnown {
		got, ok := declared[selector]
		if !ok {
			t.Errorf("%s: expected the recorded min-height %dpx to still be declared", selector, recorded)
			continue
		}
		if got < recorded {
			t.Errorf("%s: min-height shrank to %dpx, below the recorded %dpx exception", selector, got, recorded)
		}
	}
}
