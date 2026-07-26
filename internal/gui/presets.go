package gui

// presets.go - named performance presets and AC/battery automation UI.

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
	box, summary := statusCard("POWER AUTOMATION")
	w.presetSummary = summary
	box.Append(w.buildToggle("Automation", "Apply the assigned preset when the power source changes", &w.automationSwitch, func(enabled bool) {
		w.setPresetPolicy(enabled, "", "")
	}))
	w.presetDetail = gtk.NewLabel("—")
	w.presetDetail.SetHAlign(gtk.AlignStart)
	w.presetDetail.SetWrap(true)
	w.presetDetail.AddCSSClass("card-sub")
	box.Append(w.presetDetail)
	w.presetsBtn = gtk.NewButtonWithLabel("Manage automation")
	w.presetsBtn.AddCSSClass("action-btn")
	w.presetsBtn.ConnectClicked(func() { w.showPresetsView() })
	box.Append(w.presetsBtn)
	return box
}

func viewHeader(title string, back func()) (*gtk.Box, *gtk.Button, *gtk.Label) {
	header := gtk.NewBox(gtk.OrientationHorizontal, 8)
	header.SetMarginTop(10)
	header.SetMarginBottom(6)
	header.SetMarginStart(14)
	backBtn := gtk.NewButton()
	backBtn.SetIconName("go-previous-symbolic")
	backBtn.AddCSSClass("view-back-btn")
	backBtn.ConnectClicked(back)
	header.Append(backBtn)
	label := gtk.NewLabel(title)
	label.SetHAlign(gtk.AlignStart)
	label.AddCSSClass("drawer-title")
	header.Append(label)
	return header, backBtn, label
}

func (w *Window) buildPresetsView() *gtk.Box {
	view := gtk.NewBox(gtk.OrientationVertical, 0)
	header, back, _ := viewHeader("Power Automation", func() { w.showMainView() })
	w.presetsBackBtn = back
	view.Append(header)

	content := gtk.NewBox(gtk.OrientationVertical, 8)
	content.SetMarginTop(4)
	content.SetMarginBottom(12)
	content.SetMarginStart(12)
	content.SetMarginEnd(12)

	w.acAssignment, w.acAssignmentLabel, w.acChangeBtn = w.buildAssignmentCard("WHEN PLUGGED IN", "ac")
	w.batteryAssignment, w.batteryAssignLabel, w.batteryChangeBtn = w.buildAssignmentCard("ON BATTERY", "battery")
	content.Append(w.acAssignment)
	content.Append(w.batteryAssignment)

	content.Append(sectionLabel("SAVED PRESETS"))
	w.presetsList = gtk.NewBox(gtk.OrientationVertical, 6)
	content.Append(w.presetsList)

	create := plainPowerCard("CREATE PRESET")
	explanation := gtk.NewLabel("Saves your current settings as a preset. Charge limit isn't included.")
	explanation.SetHAlign(gtk.AlignStart)
	explanation.SetWrap(true)
	explanation.AddCSSClass("card-sub")
	create.Append(explanation)
	w.presetCurrent = gtk.NewLabel("—")
	w.presetCurrent.SetHAlign(gtk.AlignStart)
	w.presetCurrent.SetWrap(true)
	w.presetCurrent.AddCSSClass("preset-summary")
	create.Append(w.presetCurrent)
	w.presetNameEntry = gtk.NewEntry()
	w.presetNameEntry.SetPlaceholderText("Preset name")
	create.Append(w.presetNameEntry)
	w.presetSaveBtn = gtk.NewButtonWithLabel("Create Preset")
	w.presetSaveBtn.AddCSSClass("suggested-action")
	w.presetSaveBtn.ConnectClicked(func() {
		name := strings.TrimSpace(w.presetNameEntry.Text())
		if name == "" {
			w.setPresetStatus("Enter a preset name")
			return
		}
		if _, exists := w.state.Presets[name]; exists {
			w.setPresetStatus("A preset with that name already exists. Use Update on its card.")
			return
		}
		w.runStateAction("create preset", func() (bool, error) { return api.SendPresetSave(name) })
	})
	create.Append(w.presetSaveBtn)
	content.Append(create)

	restore := gtk.NewButtonWithLabel("Restore Recommended Automation")
	w.presetRestoreBtn = restore
	restore.AddCSSClass("action-btn")
	restore.ConnectClicked(func() {
		w.showConfirmation(
			"Restore Recommended Automation",
			"Create or refresh Plugged In and On Battery, assign them to the correct power sources, and enable automation? Other presets and your battery charge limit are preserved.",
			"Restore",
			"presets",
			false,
			func() { w.runStateAction("restore recommended automation", api.SendRestoreRecommended) },
		)
	})
	content.Append(restore)
	w.presetStatus = gtk.NewLabel("")
	w.presetStatus.SetHAlign(gtk.AlignStart)
	w.presetStatus.SetWrap(true)
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

func (w *Window) buildAssignmentCard(title, target string) (*gtk.Box, *gtk.Label, *gtk.Button) {
	card := plainPowerCard(title)
	summary := gtk.NewLabel("Not assigned")
	summary.SetHAlign(gtk.AlignStart)
	summary.SetWrap(true)
	summary.AddCSSClass("preset-summary")
	card.Append(summary)
	change := gtk.NewButtonWithLabel("Change")
	change.AddCSSClass("action-btn")
	change.ConnectClicked(func() { w.showPresetChooser(target) })
	card.Append(change)
	return card, summary, change
}

func plainPowerCard(title string) *gtk.Box {
	card, pill := statusCard(title)
	pill.SetVisible(false)
	return card
}

func (w *Window) showPresetsView() {
	if w.viewStack == nil {
		return
	}
	if w.presetsScroll == nil {
		w.viewStack.AddNamed(w.buildPresetsView(), "presets")
	}
	w.syncPresets()
	w.viewStack.SetVisibleChildName("presets")
	w.swapFocusList(w.presetFocusItems)
}

func (w *Window) syncPresets() {
	if w.state == nil {
		return
	}
	enabled := currentPolicyEnabled(w.state)
	status, detail := powerPolicySummary(w.state)
	if w.presetSummary != nil {
		w.presetSummary.SetLabel(status)
		w.presetDetail.SetLabel(detail)
		if w.automationSwitch != nil {
			w.automationSwitch.SetActive(enabled)
		}
	}
	if w.acAssignment == nil {
		return
	}
	if enabled {
		w.acAssignment.RemoveCSSClass("automation-off")
		w.batteryAssignment.RemoveCSSClass("automation-off")
	} else {
		w.acAssignment.AddCSSClass("automation-off")
		w.batteryAssignment.AddCSSClass("automation-off")
	}
	w.acChangeBtn.SetSensitive(enabled)
	w.batteryChangeBtn.SetSensitive(enabled)
	policy := w.state.PowerPolicy
	ac, battery := "", ""
	if policy != nil {
		ac, battery = policy.ACPreset, policy.BatteryPreset
	}
	w.acAssignmentLabel.SetLabel(assignmentSummary(ac, w.state.Presets))
	w.batteryAssignLabel.SetLabel(assignmentSummary(battery, w.state.Presets))
	w.presetCurrent.SetLabel(currentSettingsSummary(w.state))
	w.rebuildPresetRows()
	if w.viewStack != nil && w.viewStack.VisibleChildName() == "presets" {
		w.swapFocusList(w.presetFocusItems)
	}
}

func (w *Window) rebuildPresetRows() {
	if w.presetsList == nil || w.state == nil {
		return
	}
	for child := w.presetsList.FirstChild(); child != nil; child = w.presetsList.FirstChild() {
		w.presetsList.Remove(child)
	}
	w.presetFocusItems = []focusItem{{
		widget: w.presetsBackBtn, row: 0, col: 0, section: "automation-header",
		onActivate: func() { w.presetsBackBtn.Activate() },
	}}
	row := 1
	w.presetFocusItems = append(w.presetFocusItems,
		focusItem{widget: w.acChangeBtn, row: row, col: 0, section: "assignments", onActivate: func() { w.acChangeBtn.Activate() }},
		focusItem{widget: w.batteryChangeBtn, row: row + 1, col: 0, section: "assignments", onActivate: func() { w.batteryChangeBtn.Activate() }},
	)
	row += 2

	for _, name := range sortedPresetNames(w.state.Presets) {
		name := name
		preset := w.state.Presets[name]
		card := plainPowerCard(name)
		usage := gtk.NewLabel(presetUsage(name, w.state))
		usage.SetHAlign(gtk.AlignStart)
		usage.AddCSSClass("card-sub")
		card.Append(usage)
		summary := gtk.NewLabel(presetSummary(preset))
		summary.SetHAlign(gtk.AlignStart)
		summary.SetWrap(true)
		summary.AddCSSClass("preset-summary")
		card.Append(summary)
		actions := gtk.NewBox(gtk.OrientationHorizontal, 4)
		actions.SetHomogeneous(true)
		actions.AddCSSClass("btn-group")
		apply := gtk.NewButtonWithLabel("Apply")
		apply.AddCSSClass("suggested-action")
		apply.ConnectClicked(func() { w.runStateAction("apply preset", func() (bool, error) { return api.SendPresetApply(name) }) })
		update := gtk.NewButtonWithLabel("Update")
		update.ConnectClicked(func() { w.runStateAction("update preset", func() (bool, error) { return api.SendPresetSave(name) }) })
		remove := gtk.NewButtonWithLabel("Delete")
		remove.AddCSSClass("destructive-action")
		assigned := presetAssigned(name, w.state)
		remove.SetSensitive(!assigned)
		remove.ConnectClicked(func() {
			w.showConfirmation("Delete Preset", fmt.Sprintf("Delete %q? Current hardware settings will not change.", name), "Delete", "presets", true, func() {
				w.runStateAction("delete preset", func() (bool, error) { return api.SendPresetDelete(name) })
			})
		})
		for _, button := range []*gtk.Button{apply, update, remove} {
			actions.Append(button)
		}
		card.Append(actions)
		w.presetsList.Append(card)
		for col, button := range []*gtk.Button{apply, update, remove} {
			btn := button
			w.presetFocusItems = append(w.presetFocusItems, focusItem{widget: btn, row: row, col: col, section: "saved-presets", onActivate: func() { btn.Activate() }})
		}
		row++
	}
	w.presetFocusItems = append(w.presetFocusItems,
		focusItem{widget: w.presetNameEntry, row: row, col: 0, section: "create-preset", onActivate: func() { w.presetNameEntry.GrabFocus() }},
		focusItem{widget: w.presetSaveBtn, row: row + 1, col: 0, section: "create-preset", onActivate: func() { w.presetSaveBtn.Activate() }},
		focusItem{widget: w.presetRestoreBtn, row: row + 2, col: 0, section: "restore", onActivate: func() { w.presetRestoreBtn.Activate() }},
	)
}

func (w *Window) buildPresetChooser() *gtk.Box {
	view := gtk.NewBox(gtk.OrientationVertical, 0)
	header, back, title := viewHeader("Choose Preset", func() { w.showPresetsView() })
	w.chooserTitle = title
	view.Append(header)
	w.chooserDetail = gtk.NewLabel("")
	w.chooserDetail.SetHAlign(gtk.AlignStart)
	w.chooserDetail.SetWrap(true)
	w.chooserDetail.SetMarginStart(12)
	w.chooserDetail.SetMarginEnd(12)
	w.chooserDetail.AddCSSClass("card-sub")
	view.Append(w.chooserDetail)
	w.chooserList = gtk.NewBox(gtk.OrientationVertical, 6)
	w.chooserList.SetMarginTop(4)
	w.chooserList.SetMarginBottom(12)
	w.chooserList.SetMarginStart(12)
	w.chooserList.SetMarginEnd(12)
	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(w.chooserList)
	w.chooserScroll = scroll
	view.Append(scroll)
	w.chooserFocusItems = []focusItem{{widget: back, row: 0, col: 0, section: "chooser-header", onActivate: func() { back.Activate() }}}
	return view
}

func (w *Window) showPresetChooser(target string) {
	if w.viewStack == nil || w.state == nil {
		return
	}
	if w.chooserScroll == nil {
		w.viewStack.AddNamed(w.buildPresetChooser(), "chooser")
	}
	for child := w.chooserList.FirstChild(); child != nil; child = w.chooserList.FirstChild() {
		w.chooserList.Remove(child)
	}
	label := "When Plugged In"
	if target == "battery" {
		label = "On Battery"
	}
	w.chooserTitle.SetLabel(label)
	appliesNow := currentPolicyEnabled(w.state) &&
		((target == "ac" && w.state.PowerSource == "ac") || (target == "battery" && w.state.PowerSource == "battery"))
	if appliesNow {
		w.chooserDetail.SetLabel("Choosing a preset assigns it here and applies it now.")
	} else {
		w.chooserDetail.SetLabel("Choosing a preset changes this automatic assignment.")
	}
	w.chooserFocusItems = w.chooserFocusItems[:1]
	for i, name := range sortedPresetNames(w.state.Presets) {
		name := name
		button := gtk.NewButtonWithLabel(name + "\n" + presetSummary(w.state.Presets[name]))
		selected := ""
		if w.state.PowerPolicy != nil {
			if target == "ac" {
				selected = w.state.PowerPolicy.ACPreset
			} else {
				selected = w.state.PowerPolicy.BatteryPreset
			}
		}
		if name == selected {
			button.AddCSSClass("active")
		}
		button.ConnectClicked(func() {
			if target == "ac" {
				w.setPresetPolicy(currentPolicyEnabled(w.state), name, "")
			} else {
				w.setPresetPolicy(currentPolicyEnabled(w.state), "", name)
			}
			w.showPresetsView()
		})
		w.chooserList.Append(button)
		w.chooserFocusItems = append(w.chooserFocusItems, focusItem{widget: button, row: i + 1, col: 0, section: "chooser", onActivate: func() { button.Activate() }})
	}
	w.viewStack.SetVisibleChildName("chooser")
	w.swapFocusList(w.chooserFocusItems)
}

func (w *Window) buildConfirmationView() *gtk.Box {
	view := gtk.NewBox(gtk.OrientationVertical, 0)
	header, back, title := viewHeader("Confirm", func() { w.returnFromConfirmation() })
	w.confirmTitle = title
	view.Append(header)
	content := gtk.NewBox(gtk.OrientationVertical, 8)
	content.SetMarginTop(8)
	content.SetMarginStart(12)
	content.SetMarginEnd(12)
	w.confirmMessage = gtk.NewLabel("")
	w.confirmMessage.SetHAlign(gtk.AlignStart)
	w.confirmMessage.SetWrap(true)
	w.confirmMessage.SetMaxWidthChars(38)
	w.confirmMessage.AddCSSClass("confirm-message")
	content.Append(w.confirmMessage)
	w.confirmBtn = gtk.NewButtonWithLabel("Confirm")
	w.confirmBtn.ConnectClicked(func() {
		action := w.confirmAction
		w.returnFromConfirmation()
		if action != nil {
			action()
		}
	})
	cancel := gtk.NewButtonWithLabel("Cancel")
	cancel.ConnectClicked(func() { w.returnFromConfirmation() })
	actions := gtk.NewBox(gtk.OrientationHorizontal, 4)
	actions.SetHomogeneous(true)
	actions.AddCSSClass("btn-group")
	actions.Append(cancel)
	actions.Append(w.confirmBtn)
	content.Append(actions)
	view.Append(content)
	w.confirmFocusItems = []focusItem{
		{widget: back, row: 0, col: 0, section: "confirm-header", onActivate: func() { back.Activate() }},
		{widget: cancel, row: 1, col: 0, section: "confirm", onActivate: func() { cancel.Activate() }},
		{widget: w.confirmBtn, row: 1, col: 1, section: "confirm", onActivate: func() { w.confirmBtn.Activate() }},
	}
	return view
}

func (w *Window) showConfirmation(title, message, actionLabel, returnView string, destructive bool, action func()) {
	if w.viewStack == nil {
		return
	}
	if w.confirmMessage == nil {
		w.viewStack.AddNamed(w.buildConfirmationView(), "confirm")
	}
	w.confirmTitle.SetLabel(title)
	w.confirmMessage.SetLabel(message)
	w.confirmBtn.SetLabel(actionLabel)
	if destructive {
		w.confirmBtn.AddCSSClass("destructive-action")
		w.confirmBtn.RemoveCSSClass("suggested-action")
	} else {
		w.confirmBtn.RemoveCSSClass("destructive-action")
		w.confirmBtn.AddCSSClass("suggested-action")
	}
	w.confirmReturnView = returnView
	w.confirmAction = action
	w.viewStack.SetVisibleChildName("confirm")
	w.swapFocusList(w.confirmFocusItems)
}

func (w *Window) returnFromConfirmation() {
	returnView := w.confirmReturnView
	w.confirmAction = nil
	w.confirmReturnView = ""
	if returnView == "custom" {
		w.viewStack.SetVisibleChildName("custom")
		w.swapFocusList(w.customFocusItems)
		return
	}
	w.showPresetsView()
}

func sortedPresetNames(presets map[string]api.Preset) []string {
	names := make([]string, 0, len(presets))
	for name := range presets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func currentPolicyEnabled(state *api.State) bool {
	return state != nil && state.PowerPolicy != nil && state.PowerPolicy.Enabled
}

func powerPolicySummary(state *api.State) (status, detail string) {
	status = "MANUAL"
	if currentPolicyEnabled(state) {
		status = "AUTOMATIC"
	}
	if state == nil || state.PowerPolicy == nil {
		return status, "No presets assigned to AC/battery"
	}
	detail = fmt.Sprintf("AC: %s · Battery: %s",
		assignmentSummary(state.PowerPolicy.ACPreset, state.Presets),
		assignmentSummary(state.PowerPolicy.BatteryPreset, state.Presets))
	if state.ActivePreset != "" {
		active := state.ActivePreset
		if state.PowerSource != "" {
			active += " (" + strings.ToUpper(state.PowerSource) + ")"
		}
		detail += "\nActive now: " + active
	}
	return status, detail
}

// ruleEffect renders one side of the automation rule: the preset's headline
// profile when known (the useful behavior), falling back to its name.
func ruleEffect(name string, presets map[string]api.Preset) string {
	if name == "" {
		return "Not assigned"
	}
	if p, ok := presets[name]; ok {
		if prof := strings.TrimSpace(p.Profile); prof != "" {
			return strings.Title(prof) //nolint:staticcheck // ASCII daemon value
		}
	}
	return name
}

func assignmentSummary(name string, presets map[string]api.Preset) string {
	if name == "" {
		return "Not assigned"
	}
	effect := ruleEffect(name, presets)
	if effect == name || effect == "Not assigned" {
		return name
	}
	return fmt.Sprintf("%s → %s", name, effect)
}

func presetUsage(name string, state *api.State) string {
	if state == nil {
		return "Not assigned"
	}
	var uses []string
	if state.ActivePreset == name {
		uses = append(uses, "ACTIVE")
	}
	if state.PowerPolicy != nil {
		if state.PowerPolicy.ACPreset == name {
			uses = append(uses, "WHEN PLUGGED IN")
		}
		if state.PowerPolicy.BatteryPreset == name {
			uses = append(uses, "ON BATTERY")
		}
	}
	if len(uses) == 0 {
		return "Not assigned"
	}
	return strings.Join(uses, " · ")
}

func presetAssigned(name string, state *api.State) bool {
	return state != nil && state.PowerPolicy != nil &&
		(state.PowerPolicy.ACPreset == name || state.PowerPolicy.BatteryPreset == name)
}

func presetSummary(preset api.Preset) string {
	profile := strings.Title(preset.Profile) //nolint:staticcheck // ASCII daemon value
	if profile == "" {
		profile = "Default"
	}
	fan := "Auto"
	if preset.FanCurve != nil {
		fan = strings.Title(matchingFanPreset(preset.FanCurve)) //nolint:staticcheck // ASCII label
		if fan == "" {
			fan = "Custom"
		}
	}
	tdp := "Default"
	if preset.TDP != nil {
		tdp = fmt.Sprintf("%d/%d/%d W", preset.TDP.PL1SPL, preset.TDP.PL2SPPT, preset.TDP.FPPT)
	}
	uv := "Stock"
	if preset.Undervolt != nil {
		uv = fmt.Sprintf("%d", preset.Undervolt.CPUCO)
	}
	cpu := "Default"
	if preset.CPUPower != nil {
		boost := "off"
		if preset.CPUPower.Boost {
			boost = "on"
		}
		cpu = fmt.Sprintf("%d MHz min · %s · Boost %s", preset.CPUPower.MinFrequencyKHz/1000, eppDisplayName(preset.CPUPower.EPP), boost)
	}
	refresh := "Default"
	if preset.RefreshRate > 0 {
		refresh = fmt.Sprintf("%d Hz", preset.RefreshRate)
	}
	overdrive := "Default"
	if preset.PanelOverdrive != nil {
		if *preset.PanelOverdrive == 1 {
			overdrive = "On"
		} else {
			overdrive = "Off"
		}
	}
	rows := []string{
		"Profile: " + profile,
		"Fan: " + fan,
		"TDP: " + tdp,
		"Undervolt: " + uv,
		"CPU: " + cpu,
		"Refresh: " + refresh,
		"Overdrive: " + overdrive,
	}
	return strings.Join(rows, "\n")
}

func eppDisplayName(epp string) string {
	switch epp {
	case "balance_performance":
		return "Responsive CPU"
	case "balance_power":
		return "Efficient CPU"
	case "performance":
		return "Maximum performance"
	case "power":
		return "Maximum efficiency"
	default:
		return strings.ReplaceAll(strings.Title(epp), "_", " ") //nolint:staticcheck // ASCII daemon value
	}
}

func currentSettingsSummary(state *api.State) string {
	if state == nil {
		return "Current settings unavailable"
	}
	preset := api.Preset{
		Profile:        state.Profile,
		CPUPower:       nil,
		RefreshRate:    state.RefreshRate,
		PanelOverdrive: &state.PanelOverdrive,
	}
	if state.FanCurveActive {
		preset.FanCurve = state.FanCurve
	}
	if state.TDPActive {
		preset.TDP = state.TDP
	}
	if state.UndervoltActive {
		preset.Undervolt = state.Undervolt
	}
	if state.CPUPower != nil {
		preset.CPUPower = &api.CPUPowerPreset{MinFrequencyKHz: state.CPUPower.MinFrequencyKHz, EPP: state.CPUPower.EPP, Boost: state.CPUPower.Boost}
	}
	return presetSummary(preset)
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
	w.runStateAction("set power automation", func() (bool, error) {
		if ac == "" || battery == "" {
			ok, latest, err := api.SendGetState()
			if err != nil || !ok {
				return ok, err
			}
			if latest.PowerPolicy != nil {
				if ac == "" {
					ac = latest.PowerPolicy.ACPreset
				}
				if battery == "" {
					battery = latest.PowerPolicy.BatteryPreset
				}
			}
		}
		return api.SendPowerPolicySet(enabled, ac, battery)
	})
}

type stateAction struct {
	name         string
	fn           func() (bool, error)
	reportStatus bool
	onApplied    func()
	onFailed     func()
}

func (w *Window) runStateAction(action string, fn func() (bool, error)) {
	w.queueStateAction(stateAction{name: action, fn: fn, reportStatus: true})
}

func (w *Window) runStateActionQuiet(action string, fn func() (bool, error), onApplied, onFailed func()) {
	w.queueStateAction(stateAction{name: action, fn: fn, onApplied: onApplied, onFailed: onFailed})
}

func (w *Window) queueStateAction(action stateAction) {
	if w.stateActionBusy {
		w.stateActionQueue = append(w.stateActionQueue, action)
		if action.reportStatus {
			w.setPresetStatus("Action queued...")
		}
		return
	}
	w.startStateAction(action)
}

func (w *Window) startStateAction(action stateAction) {
	w.stateActionBusy = true
	w.stateRequestGen++ // invalidate state reads started before this mutation
	if action.reportStatus {
		w.setPresetStatus("Working...")
	}
	go func() {
		handled, err := action.fn()
		if err == nil && !handled {
			err = fmt.Errorf("z13ctl daemon is not running")
		}
		if err != nil {
			slog.Warn(action.name+" failed", "err", err)
			glib.IdleAdd(func() {
				if action.onFailed != nil {
					action.onFailed()
				}
				if action.reportStatus {
					w.setPresetStatus(err.Error())
				}
				w.syncState()
				w.finishStateAction()
			})
			return
		}
		ok, state, err := api.SendGetState()
		if err != nil || !ok {
			slog.Warn(action.name+" applied but state refresh failed", "err", err)
		}
		glib.IdleAdd(func() {
			if err != nil || !ok {
				if action.onApplied != nil {
					action.onApplied()
				}
				if action.reportStatus {
					w.setPresetStatus("Applied, but failed to refresh state")
				}
				w.finishStateAction()
				return
			}
			if action.reportStatus {
				w.setPresetStatus("")
			}
			w.state = state
			w.syncState()
			if action.onApplied != nil {
				action.onApplied()
			}
			w.finishStateAction()
		})
	}()
}

func (w *Window) finishStateAction() {
	w.stateActionBusy = false
	if len(w.stateActionQueue) == 0 {
		return
	}
	next := w.stateActionQueue[0]
	w.stateActionQueue = w.stateActionQueue[1:]
	w.startStateAction(next)
}

func (w *Window) setPresetStatus(message string) {
	if w.presetStatus != nil {
		w.presetStatus.SetLabel(message)
	}
}
