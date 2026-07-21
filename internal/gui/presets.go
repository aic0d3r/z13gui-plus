package gui

// presets.go - named performance presets and AC/battery assignment UI.

import (
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strings"

	"github.com/dahui/z13ctl/api"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func (w *Window) buildPresetEntrySection() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 4)
	box.AddCSSClass("btn-group")
	box.Append(sectionLabel("PRESETS"))
	w.presetsBtn = gtk.NewButtonWithLabel("Manage presets")
	w.presetsBtn.ConnectClicked(func() { w.showPresetsView() })
	box.Append(w.presetsBtn)
	return box
}

func (w *Window) buildPresetsView() *gtk.Box {
	view := gtk.NewBox(gtk.OrientationVertical, 0)
	w.presetsBackBtn = gtk.NewButton()
	w.presetsBackBtn.SetIconName("go-previous-symbolic")
	w.presetsBackBtn.AddCSSClass("view-back-btn")
	w.presetsBackBtn.ConnectClicked(func() { w.showMainView() })

	header := gtk.NewBox(gtk.OrientationHorizontal, 8)
	header.SetMarginTop(10)
	header.SetMarginBottom(6)
	header.SetMarginStart(14)
	header.Append(w.presetsBackBtn)
	title := gtk.NewLabel("Power Presets")
	title.SetHAlign(gtk.AlignStart)
	title.AddCSSClass("drawer-title")
	header.Append(title)
	view.Append(header)

	content := gtk.NewBox(gtk.OrientationVertical, 6)
	content.SetMarginTop(4)
	content.SetMarginBottom(12)
	content.SetMarginStart(12)
	content.SetMarginEnd(12)
	content.Append(w.buildToggle("AUTOMATIC AC / BATTERY", "Switch assigned presets on power-source changes", &w.presetAuto, func(enabled bool) {
		w.setPresetPolicy(enabled, "", "")
	}))

	w.presetsList = gtk.NewBox(gtk.OrientationVertical, 6)
	content.Append(w.presetsList)

	content.Append(sectionLabel("SAVE CURRENT"))
	w.presetNameEntry = gtk.NewEntry()
	w.presetNameEntry.SetPlaceholderText("Preset name")
	content.Append(w.presetNameEntry)
	w.presetSaveBtn = gtk.NewButtonWithLabel("Save current settings")
	w.presetSaveBtn.ConnectClicked(func() {
		name := strings.TrimSpace(w.presetNameEntry.Text())
		if name == "" {
			w.setPresetStatus("Enter a preset name")
			return
		}
		w.runPresetAction("save preset", func() (bool, error) { return api.SendPresetSave(name) })
	})
	content.Append(w.presetSaveBtn)

	w.presetStatus = gtk.NewLabel("")
	w.presetStatus.SetWrap(true)
	w.presetStatus.SetHAlign(gtk.AlignStart)
	content.Append(w.presetStatus)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(content)
	w.presetsScroll = scroll
	view.Append(scroll)

	w.rebuildPresetRows()
	return view
}

func (w *Window) showPresetsView() {
	if w.viewStack == nil {
		return
	}
	if w.presetsScroll == nil {
		w.viewStack.AddNamed(w.buildPresetsView(), "presets")
	}
	wasSyncing := w.syncing
	w.syncing = true
	w.syncPresets()
	w.syncing = wasSyncing
	w.viewStack.SetVisibleChildName("presets")
	w.swapFocusList(w.presetFocusItems)
}

func (w *Window) syncPresets() {
	if w.state == nil {
		return
	}
	if w.presetsBtn != nil {
		label := "Manage presets"
		if w.state.ActivePreset != "" {
			label = "Active: " + w.state.ActivePreset
		}
		w.presetsBtn.SetLabel(label)
	}
	if w.presetAuto != nil {
		enabled := w.state.PowerPolicy != nil && w.state.PowerPolicy.Enabled
		w.presetAuto.SetActive(enabled)
	}
	if w.presetsList != nil {
		w.rebuildPresetRows()
		if w.viewStack != nil && w.viewStack.VisibleChildName() == "presets" {
			w.swapFocusList(w.presetFocusItems)
		}
	}
}

func (w *Window) rebuildPresetRows() {
	if w.presetsList == nil || w.state == nil {
		return
	}
	for child := w.presetsList.FirstChild(); child != nil; child = w.presetsList.FirstChild() {
		w.presetsList.Remove(child)
	}

	names := make([]string, 0, len(w.state.Presets))
	for name := range w.state.Presets {
		names = append(names, name)
	}
	sort.Strings(names)
	w.presetFocusItems = []focusItem{{
		widget: w.presetsBackBtn, row: 0, col: 0, section: "preset-header",
		onActivate: func() { w.presetsBackBtn.Activate() },
	}}
	rowIndex := 1
	if w.presetAuto != nil {
		w.presetFocusItems = append(w.presetFocusItems, focusItem{
			widget: w.presetAuto, row: rowIndex, col: 0, section: "preset-policy",
			onActivate: func() { w.presetAuto.SetActive(!w.presetAuto.Active()) },
		})
		rowIndex++
	}

	for _, presetName := range names {
		name := presetName
		preset := w.state.Presets[name]
		box := gtk.NewBox(gtk.OrientationVertical, 3)
		box.AddCSSClass("btn-group")
		applyLabel := fmt.Sprintf("%s · %s · %d Hz", name, strings.Title(preset.Profile), preset.RefreshRate) //nolint:staticcheck // ASCII labels
		if name == w.state.ActivePreset {
			applyLabel = "ACTIVE · " + applyLabel
		}
		apply := gtk.NewButtonWithLabel(applyLabel)
		apply.ConnectClicked(func() {
			w.runPresetAction("apply preset", func() (bool, error) { return api.SendPresetApply(name) })
		})
		box.Append(apply)

		actions := gtk.NewBox(gtk.OrientationHorizontal, 4)
		actions.SetHomogeneous(true)
		actions.AddCSSClass("btn-group")
		ac := gtk.NewButtonWithLabel("AC")
		battery := gtk.NewButtonWithLabel("BAT")
		remove := gtk.NewButtonWithLabel("Delete")
		if policy := w.state.PowerPolicy; policy != nil {
			if policy.ACPreset == name {
				ac.AddCSSClass("active")
			}
			if policy.BatteryPreset == name {
				battery.AddCSSClass("active")
			}
			remove.SetSensitive(policy.ACPreset != name && policy.BatteryPreset != name)
		}
		ac.ConnectClicked(func() { w.setPresetPolicy(currentPolicyEnabled(w.state), name, "") })
		battery.ConnectClicked(func() { w.setPresetPolicy(currentPolicyEnabled(w.state), "", name) })
		remove.ConnectClicked(func() {
			w.runPresetAction("delete preset", func() (bool, error) { return api.SendPresetDelete(name) })
		})
		actions.Append(ac)
		actions.Append(battery)
		actions.Append(remove)
		box.Append(actions)
		w.presetsList.Append(box)

		for col, button := range []*gtk.Button{apply, ac, battery, remove} {
			btn := button
			w.presetFocusItems = append(w.presetFocusItems, focusItem{
				widget: btn, row: rowIndex, col: col, section: "preset-row",
				onActivate: func() { btn.Activate() },
			})
		}
		rowIndex++
	}
	if w.presetSaveBtn != nil {
		w.presetFocusItems = append(w.presetFocusItems, focusItem{
			widget: w.presetSaveBtn, row: rowIndex, col: 0, section: "preset-save",
			onActivate: func() { w.presetSaveBtn.Activate() },
		})
	}
}

func currentPolicyEnabled(state *api.State) bool {
	return state != nil && state.PowerPolicy != nil && state.PowerPolicy.Enabled
}

func presetStateChanged(old, next *api.State) bool {
	if old == nil || next == nil || old.ActivePreset != next.ActivePreset || old.PowerSource != next.PowerSource {
		return true
	}
	if !reflect.DeepEqual(old.Presets, next.Presets) {
		return true
	}
	if old.PowerPolicy == nil || next.PowerPolicy == nil {
		return old.PowerPolicy != next.PowerPolicy
	}
	return *old.PowerPolicy != *next.PowerPolicy
}

func (w *Window) setPresetPolicy(enabled bool, ac, battery string) {
	if w.state == nil {
		return
	}
	if ac == "" && w.state.PowerPolicy != nil {
		ac = w.state.PowerPolicy.ACPreset
	}
	if battery == "" && w.state.PowerPolicy != nil {
		battery = w.state.PowerPolicy.BatteryPreset
	}
	w.runPresetAction("set power policy", func() (bool, error) {
		return api.SendPowerPolicySet(enabled, ac, battery)
	})
}

func (w *Window) runPresetAction(action string, fn func() (bool, error)) {
	if w.presetBusy {
		return
	}
	w.presetBusy = true
	w.setPresetStatus("Working...")
	go func() {
		handled, err := fn()
		if err == nil && !handled {
			err = fmt.Errorf("z13ctl daemon is not running")
		}
		if err != nil {
			slog.Warn(action+" failed", "err", err)
			glib.IdleAdd(func() {
				w.presetBusy = false
				w.setPresetStatus(err.Error())
				wasSyncing := w.syncing
				w.syncing = true
				w.syncPresets()
				w.syncing = wasSyncing
			})
			return
		}
		ok, state, err := api.SendGetState()
		if err != nil || !ok {
			glib.IdleAdd(func() {
				w.presetBusy = false
				w.setPresetStatus("Applied, but failed to refresh state")
			})
			return
		}
		glib.IdleAdd(func() {
			w.presetBusy = false
			w.setPresetStatus("")
			w.state = state
			w.syncState()
		})
	}()
}

func (w *Window) setPresetStatus(message string) {
	if w.presetStatus != nil {
		w.presetStatus.SetLabel(message)
	}
}
