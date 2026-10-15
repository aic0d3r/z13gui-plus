package gui

// layout.go — hide cards that do not apply. Two sources decide visibility:
//   - hardware: the daemon reports no battery / no RGB device (desktop Strix Halo),
//     so those cards disappear on their own and cannot be switched back on;
//   - the user: Settings > Layout holds one switch per card, persisted as the
//     "hidden" list in config.toml.
//
// Cards that guard against risk (status strip, profile, fan safety banner, the
// 75 W caution) are not listed, so hiding a card never hides a warning.

import (
	"fmt"
	"strings"

	"github.com/aic0d3r/z13gui-plus/internal/theme"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type layoutCard struct{ id, tab, label string }

// layoutCards is the Layout list in display order, grouped by tab.
var layoutCards = []layoutCard{
	{"overview.power", "OVERVIEW", "APU power"},
	{"overview.fans", "OVERVIEW", "Fans"},
	{"overview.memory", "OVERVIEW", "Memory & clocks"},
	{"overview.battery", "OVERVIEW", "Battery"},
	{"power.fan", "POWER", "Fan mode"},
	{"power.boost", "POWER", "CPU Boost"},
	{"power.presets", "POWER", "Power automation"},
	{"power.battery", "POWER", "Battery strategy"},
	{"power.tuning", "POWER", "Power tuning"},
	{"tab.rgb", "TABS", "RGB tab"},
}

// registerCard remembers a card so applyLayout can show or hide it.
func (w *Window) registerCard(id string, widget gtk.Widgetter) {
	w.layoutWidgets[id] = widget
}

// hiddenCards parses the saved comma-separated list.
func hiddenCards(list string) map[string]bool {
	out := map[string]bool{}
	for _, id := range strings.Split(list, ",") {
		if id = strings.TrimSpace(id); id != "" {
			out[id] = true
		}
	}
	return out
}

// cardUnavailable reports hardware that is absent. Unknown state counts as present.
// IDs missing from layoutCards (power.profile, set.*) are hidden only by hardware.
func (w *Window) cardUnavailable(id string) bool {
	s := w.state
	if s == nil {
		return false
	}
	// Older daemons send no capabilities: treat every control as present.
	c := s.Capabilities
	switch id {
	case "overview.battery":
		return s.BatteryDetail == nil
	case "power.battery":
		return s.BatteryDetail == nil || (c != nil && !c.BatteryLimit)
	case "power.profile":
		return c != nil && !c.Profile
	case "power.fan":
		return c != nil && !c.FanCurve
	case "power.tuning":
		return c != nil && !c.TDP && !c.FanCurve && !s.UndervoltAvailable && s.CPUPower == nil
	case "set.refresh":
		return c != nil && !c.Display
	case "set.overdrive":
		return c != nil && !c.PanelOverdrive
	case "set.bootsound":
		return c != nil && !c.BootSound
	case "set.display":
		return c != nil && !c.Display && !c.PanelOverdrive && !c.BootSound
	case "power.boost":
		return s.CPUPower == nil
	case "tab.rgb":
		for _, present := range s.DevicePresence {
			if present {
				return false
			}
		}
		return s.DevicePresence != nil
	}
	return false
}

// applyLayout shows each registered card unless it is hidden or unavailable and
// syncs the Settings switches. Safe to call often.
func (w *Window) applyLayout() {
	hidden := hiddenCards(theme.LoadAppConfig().Hidden)
	prev := w.syncing
	w.syncing = true
	defer func() { w.syncing = prev }()
	for id, widget := range w.layoutWidgets {
		gtk.BaseWidget(widget).SetVisible(!hidden[id] && !w.cardUnavailable(id))
	}
	shown, off := 0, 0
	for id, sw := range w.layoutSwitches {
		syncSwitch(sw, !hidden[id])
		sw.SetSensitive(!w.cardUnavailable(id))
		switch {
		case w.cardUnavailable(id):
			off++
		case hidden[id]:
			off++
		default:
			shown++
		}
	}
	if w.layoutSummary != nil {
		w.layoutSummary.SetLabel(fmt.Sprintf("%d SHOWN · %d HIDDEN", shown, off))
	}
	if rgb := w.mainTabBtns["rgb"]; rgb != nil && !rgb.IsVisible() && w.activeTab == "rgb" {
		w.switchTab("overview")
	}
}

// setCardHidden persists one switch change and re-applies the layout.
func (w *Window) setCardHidden(id string, hide bool) {
	cfg := theme.LoadAppConfig()
	hidden := hiddenCards(cfg.Hidden)
	if hide {
		hidden[id] = true
	} else {
		delete(hidden, id)
	}
	var ids []string
	for _, c := range layoutCards { // stable order keeps config.toml diff-friendly
		if hidden[c.id] {
			ids = append(ids, c.id)
		}
	}
	cfg.Hidden = strings.Join(ids, ",")
	theme.SaveAppConfig(cfg)
	w.applyLayout()
}

// buildLayoutSection is the Settings card with one switch per hideable card.
func (w *Window) buildLayoutSection() *gtk.Box {
	// Ten switch rows are tall, so the list stays folded until asked for.
	card := gtk.NewBox(gtk.OrientationVertical, 6)
	card.AddCSSClass("card")
	w.layoutSummary = gtk.NewLabel("")
	w.layoutSummary.SetHAlign(gtk.AlignStart)
	w.layoutSummary.AddCSSClass("section-collapse-summary")
	header := newCollapsibleHeader("LAYOUT", w.layoutSummary, false)
	card.Append(header)
	w.layoutHeader = header
	box := gtk.NewBox(gtk.OrientationVertical, 6)
	box.SetVisible(false)
	header.ConnectToggled(func() { box.SetVisible(header.Active()) })
	card.Append(box)
	group := ""
	for _, c := range layoutCards {
		if c.tab != group {
			group = c.tab
			box.Append(sectionLabel(group))
		}
		var sw *gtk.Switch
		id := c.id
		row := w.buildToggle(c.label, "Show or hide this card. Cards for missing hardware stay hidden.", &sw, func(on bool) {
			w.setCardHidden(id, !on)
		})
		w.layoutSwitches[id] = sw
		box.Append(row)
	}
	reset := gtk.NewButtonWithLabel("Show All Cards")
	reset.AddCSSClass("action-btn")
	reset.ConnectClicked(func() {
		cfg := theme.LoadAppConfig()
		cfg.Hidden = ""
		theme.SaveAppConfig(cfg)
		w.applyLayout()
	})
	box.Append(reset)
	return card
}
