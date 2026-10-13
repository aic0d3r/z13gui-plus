package gui

// presets.go - named performance presets and AC/battery automation UI.

import (
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/dahui/z13ctl/api"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

func (w *Window) buildPresetEntrySection() *gtk.Box {
	card := gtk.NewBox(gtk.OrientationVertical, 6)
	card.AddCSSClass("card")
	header := gtk.NewBox(gtk.OrientationHorizontal, 6)
	title := gtk.NewLabel("POWER AUTOMATION")
	title.SetHAlign(gtk.AlignStart)
	title.SetHExpand(true)
	title.AddCSSClass("card-title")
	w.automationSwitch = w.newAutomationSwitch()
	header.Append(title)
	header.Append(w.automationSwitch)
	card.Append(header)

	w.automationHint = gtk.NewLabel("")
	w.automationHint.SetHAlign(gtk.AlignStart)
	w.automationHint.SetWrap(true)
	w.automationHint.AddCSSClass("card-sub")
	w.automationHint.SetVisible(false)
	card.Append(w.automationHint)

	w.presetAssignments = gtk.NewBox(gtk.OrientationVertical, 2)
	w.presetACRow, w.presetACLabel, w.presetACStatus, w.presetACDetail = automationSummaryRow("PLUGGED IN")
	w.presetBatteryRow, w.presetBatteryLabel, w.presetBatteryStatus, w.presetBatteryDetail = automationSummaryRow("ON BATTERY")
	w.presetAssignments.Append(w.presetACRow)
	w.presetAssignments.Append(w.presetBatteryRow)
	card.Append(w.presetAssignments)

	// One tap applies a preset; rebuilt when the preset names change.
	w.presetChips = gtk.NewGrid()
	w.presetChips.SetColumnSpacing(4)
	w.presetChips.SetRowSpacing(4)
	w.presetChips.SetColumnHomogeneous(true)
	w.presetChips.AddCSSClass("btn-group")
	card.Append(w.presetChips)

	// Shown while a manual change overrides the assigned preset.
	w.presetReapplyBtn = gtk.NewButtonWithLabel("Reapply")
	w.presetReapplyBtn.AddCSSClass("outline-btn")
	w.presetReapplyBtn.SetHAlign(gtk.AlignStart)
	w.presetReapplyBtn.SetVisible(false)
	w.presetReapplyBtn.ConnectClicked(func() {
		if name, _ := automationCurrent(w.state); name != "" {
			w.applyPresetNow(name)
		}
	})
	card.Append(w.presetReapplyBtn)

	w.presetsBtn = gtk.NewButtonWithLabel("Manage Presets")
	w.presetsBtn.AddCSSClass("action-btn")
	w.presetsBtn.ConnectClicked(func() { w.showPresetsView() })
	card.Append(w.presetsBtn)
	return card
}

func (w *Window) applyPresetNow(name string) {
	w.queueStateAction(stateAction{
		name: "apply preset", reportStatus: true,
		fn:        func() (bool, error) { return api.SendPresetApply(name) },
		onApplied: func() { w.showPresetFeedback(name + " applied") },
	})
}

// syncPresetChips rebuilds the chip row when preset names change and marks
// the active preset.
func (w *Window) syncPresetChips() {
	if w.presetChips == nil || w.state == nil {
		return
	}
	names := sortedPresetNames(w.state.Presets)
	if !slices.Equal(names, w.presetChipNames) {
		for child := w.presetChips.FirstChild(); child != nil; child = w.presetChips.FirstChild() {
			w.presetChips.Remove(child)
		}
		w.presetChipNames = names
		w.presetChipBtns = make(map[string]*gtk.Button, len(names))
		w.presetChipCols = chipColumns(names)
		for i, name := range names {
			btn := presetChip(name)
			btn.ConnectClicked(func() {
				// Highlight immediately: the tapped chip is where the eye is.
				setActiveButton(w.presetChipBtns, name)
				w.applyPresetNow(name)
			})
			w.presetChipBtns[name] = btn
			w.presetChips.Attach(btn, i%w.presetChipCols, i/w.presetChipCols, 1, 1)
		}
		w.presetChips.SetVisible(len(names) > 0)
		w.buildMainFocusList()
		if w.viewStack != nil && w.viewStack.VisibleChildName() == "main" {
			w.swapFocusList(w.mainFocusItems)
		}
	}
	for name, btn := range w.presetChipBtns {
		preset := w.state.Presets[name]
		btn.SetTooltipText(compactPresetSummary(preset))
		high := presetHighTDP(preset)
		toggleClass(btn, "high-tdp", high)
		if high {
			btn.SetTooltipText(compactPresetSummary(preset) + " — locks fans to safety cooling while active")
		}
	}
	setActiveButton(w.presetChipBtns, w.state.ActivePreset)
}

// chipColumns uses three chips per row, or two when any name would be cut
// short at a third of the 320 px drawer.
func chipColumns(names []string) int {
	for _, name := range names {
		if len([]rune(name)) > 10 {
			return 2
		}
	}
	return 3
}

// presetChip is a chip button with an ellipsized label. No width cap: the label
// ellipsizes only when the chip cell is really too narrow, and chipColumns already
// drops to two columns for long names. A hard cap truncated user-chosen names with
// no touch-accessible way to read them.
func presetChip(name string) *gtk.Button {
	label := gtk.NewLabel(name)
	label.SetEllipsize(pango.EllipsizeEnd)
	btn := gtk.NewButton()
	btn.SetChild(label)
	btn.SetTooltipText(name)
	btn.SetHExpand(true)
	btn.AddCSSClass("preset-chip")
	return btn
}

func (w *Window) newAutomationSwitch() *gtk.Switch {
	sw := gtk.NewSwitch()
	sw.AddCSSClass("automation-switch")
	sw.SetVAlign(gtk.AlignCenter)
	sw.SetTooltipText("Apply assigned presets when the power source changes")
	sw.ConnectStateSet(func(enabled bool) bool {
		if !w.syncing {
			message := "Power automation paused"
			if enabled {
				message = "Power automation enabled"
			}
			w.setPresetPolicy(enabled, "", "", message)
		}
		return false
	})
	if w.gamescope {
		addTouchActivate(sw, func() { sw.SetActive(!sw.Active()) })
	}
	return sw
}

// automationSummaryRow builds a two-line row: source, preset name and status
// pill, then the preset's key values.
func automationSummaryRow(source string) (row *gtk.Box, preset, status, detail *gtk.Label) {
	row = gtk.NewBox(gtk.OrientationVertical, 0)
	row.AddCSSClass("automation-summary-row")
	line := gtk.NewBox(gtk.OrientationHorizontal, 6)
	sourceLabel := gtk.NewLabel(source)
	sourceLabel.SetHAlign(gtk.AlignStart)
	sourceLabel.AddCSSClass("automation-source")
	line.Append(sourceLabel)
	preset = gtk.NewLabel("Not assigned")
	preset.SetHAlign(gtk.AlignEnd)
	preset.SetHExpand(true)
	preset.AddCSSClass("preset-summary")
	line.Append(preset)
	status = gtk.NewLabel("Active")
	status.AddCSSClass("pill")
	status.SetVisible(false)
	line.Append(status)
	row.Append(line)
	detail = gtk.NewLabel("")
	detail.SetHAlign(gtk.AlignEnd)
	detail.SetWrap(true)
	detail.AddCSSClass("automation-detail")
	row.Append(detail)
	return row, preset, status, detail
}

func viewHeader(title string, back func()) (*gtk.Box, *gtk.Button, *gtk.Label) {
	header := gtk.NewBox(gtk.OrientationHorizontal, 8)
	header.SetMarginTop(10)
	header.SetMarginBottom(6)
	header.SetMarginStart(14)
	backBtn := gtk.NewButton()
	backBtn.SetIconName("go-previous-symbolic")
	backBtn.AddCSSClass("view-back-btn")
	backBtn.SetTooltipText("Back")
	backBtn.ConnectClicked(back)
	header.Append(backBtn)
	label := gtk.NewLabel(title)
	label.SetHAlign(gtk.AlignStart)
	label.AddCSSClass("drawer-title")
	header.Append(label)
	return header, backBtn, label
}

// buildPresetsView is the single preset management screen: every preset with
// Apply/Edit and its AC/battery assignment, plus save-current and restore.
func (w *Window) buildPresetsView() *gtk.Box {
	view := gtk.NewBox(gtk.OrientationVertical, 0)
	header, back, title := viewHeader("Presets", func() { w.showMainView() })
	w.presetsBackBtn = back
	title.SetHExpand(true)
	w.automationViewSw = w.newAutomationSwitch()
	header.Append(w.automationViewSw)
	view.Append(header)

	content := gtk.NewBox(gtk.OrientationVertical, 8)
	content.SetMarginTop(4)
	content.SetMarginBottom(12)
	content.SetMarginStart(12)
	content.SetMarginEnd(12)

	w.presetsList = gtk.NewBox(gtk.OrientationVertical, 8)
	content.Append(w.presetsList)

	w.presetSaveBtn = gtk.NewButtonWithLabel("Save Current Settings as Preset")
	w.presetSaveBtn.AddCSSClass("action-btn")
	w.presetSaveBtn.SetTooltipText("Saves profile, TDP, undervolt, CPU, refresh and overdrive. Rename it in Edit.")
	w.presetSaveBtn.ConnectClicked(func() {
		name := autoPresetName(w.state)
		w.queueStateAction(stateAction{
			name: "save preset", reportStatus: true,
			fn:        func() (bool, error) { return api.SendPresetSave(name) },
			onApplied: func() { w.showPresetFeedback("Saved as " + name) },
		})
	})
	content.Append(w.presetSaveBtn)

	w.presetRestoreBtn = gtk.NewButtonWithLabel("Restore recommended presets")
	w.presetRestoreBtn.AddCSSClass("subtle-btn")
	w.presetRestoreBtn.ConnectClicked(func() {
		w.showConfirmation(
			"Restore Recommended Presets",
			"Reset Plugged In and On Battery to recommended values, assign them, and turn automation on. Your other presets stay.",
			"Restore",
			"presets",
			false,
			func() { w.runStateAction("restore recommended automation", api.SendRestoreRecommended) },
		)
	})
	content.Append(w.presetRestoreBtn)

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
	w.presetDetailName = ""
	w.syncPresets()
	w.viewStack.SetVisibleChildName("presets")
	w.swapFocusList(w.presetFocusItems)
}

func (w *Window) syncPresets() {
	if w.state == nil {
		return
	}
	enabled := currentPolicyEnabled(w.state)
	ac, battery := powerPolicyAssignments(w.state)
	if w.automationSwitch != nil {
		syncSwitch(w.automationSwitch, enabled)
	}
	if w.automationViewSw != nil {
		syncSwitch(w.automationViewSw, enabled)
	}

	acActive := enabled && w.state.PowerSource == "ac"
	batteryActive := enabled && w.state.PowerSource == "battery"
	current, manual := automationCurrent(w.state)
	if w.automationHint != nil {
		hint := ""
		if next := assignedForSource(w.state); !enabled && next != "" {
			hint = "Turning on applies " + next + " now."
		} else if manual {
			hint = "Manual changes active. " + current + " returns on the next plug change."
		}
		w.automationHint.SetLabel(hint)
		w.automationHint.SetVisible(hint != "")
		w.presetReapplyBtn.SetLabel("Reapply " + current)
		w.presetReapplyBtn.SetVisible(manual)
	}
	if w.presetAssignments != nil {
		w.presetACLabel.SetLabel(ac)
		w.presetBatteryLabel.SetLabel(battery)
		w.presetACDetail.SetLabel(presetDetailLine(w.state, ac))
		w.presetBatteryDetail.SetLabel(presetDetailLine(w.state, battery))
		for _, r := range []struct {
			row    *gtk.Box
			name   string
			active bool
			pill   *gtk.Label
		}{{w.presetACRow, ac, acActive, w.presetACStatus}, {w.presetBatteryRow, battery, batteryActive, w.presetBatteryStatus}} {
			toggleClass(r.row, "assignment-missing", r.name == "Not assigned")
			toggleClass(r.row, "current-assignment", r.active)
			setAssignmentPill(r.pill, r.active, manual)
		}
		toggleClass(w.presetAssignments, "automation-off", !enabled)
	}
	w.syncPresetChips()
	w.rebuildPresetRows()
	// Rebuilding the editor discards unsaved edits, so only do it when the
	// preset itself or its usage changed underneath.
	if w.presetDetailName != "" && w.presetDetailContent != nil {
		preset, ok := w.state.Presets[w.presetDetailName]
		base := clonePresetValue(preset)
		base.BaseProfile = ""
		usage := presetUsage(w.presetDetailName, w.state)
		if !ok || !reflect.DeepEqual(base, w.presetDraftBase) || usage != w.presetDetailUsage {
			w.rebuildPresetDetail()
		}
	}
	if w.viewStack != nil {
		switch w.viewStack.VisibleChildName() {
		case "presets":
			w.swapFocusList(w.presetFocusItems)
		case "preset-detail":
			w.swapFocusList(w.presetDetailFocusItems)
		}
	}
}

func toggleClass(widget gtk.Widgetter, class string, on bool) {
	base := gtk.BaseWidget(widget)
	if on {
		base.AddCSSClass(class)
	} else {
		base.RemoveCSSClass(class)
	}
}

func (w *Window) rebuildPresetRows() {
	if w.presetsList == nil || w.state == nil {
		return
	}
	for child := w.presetsList.FirstChild(); child != nil; child = w.presetsList.FirstChild() {
		w.presetsList.Remove(child)
	}
	w.presetFocusItems = []focusItem{
		{widget: w.presetsBackBtn, row: 0, col: 0, section: "presets-header", onActivate: func() { w.presetsBackBtn.Activate() }},
		{widget: w.automationViewSw, row: 0, col: 1, section: "presets-header", onActivate: func() { w.automationViewSw.SetActive(!w.automationViewSw.Active()) }},
	}
	enabled := currentPolicyEnabled(w.state)
	row := 1
	focus := func(btn *gtk.Button, col int) {
		w.presetFocusItems = append(w.presetFocusItems, focusItem{
			widget: btn, row: row, col: col, section: "presets",
			isVisible: btn.Sensitive, onActivate: func() { btn.Activate() },
		})
	}
	for _, name := range sortedPresetNames(w.state.Presets) {
		preset := w.state.Presets[name]
		card := gtk.NewBox(gtk.OrientationVertical, 6)
		card.AddCSSClass("card")
		card.AddCSSClass("preset-card")
		toggleClass(card, "current-assignment", w.state.ActivePreset == name)

		active := w.state.ActivePreset == name
		ac := w.state.PowerPolicy != nil && w.state.PowerPolicy.ACPreset == name
		battery := w.state.PowerPolicy != nil && w.state.PowerPolicy.BatteryPreset == name

		// Name left; state pills right (assignment is information here, the
		// toggles below are the action).
		header := gtk.NewBox(gtk.OrientationHorizontal, 4)
		title := gtk.NewLabel(name)
		title.SetHAlign(gtk.AlignStart)
		title.SetHExpand(true)
		title.SetEllipsize(pango.EllipsizeEnd)
		title.SetTooltipText(name) // full name when the row is too narrow for it
		title.SetXAlign(0)
		title.AddCSSClass("preset-name")
		header.Append(title)
		if active {
			header.Append(pill("Active", "accent"))
		}
		card.Append(header)
		summary := gtk.NewLabel(compactPresetSummary(preset))
		summary.SetHAlign(gtk.AlignStart)
		summary.SetWrap(true)
		summary.SetXAlign(0)
		summary.AddCSSClass("preset-summary")
		if presetHighTDP(preset) {
			summary.AddCSSClass("warning")
			summary.SetTooltipText("Sustained TDP above 75 W locks fans to safety cooling while active")
		}
		card.Append(summary)

		// One row: Apply · Edit · AC toggle · battery toggle.
		actions := gtk.NewGrid()
		actions.SetColumnSpacing(4)
		actions.SetColumnHomogeneous(true)
		actions.AddCSSClass("btn-group")
		apply := gtk.NewButtonWithLabel("Apply")
		if active {
			apply.SetLabel("Applied")
			apply.SetSensitive(false)
		}
		apply.ConnectClicked(func() { w.applyPresetNow(name) })
		edit := gtk.NewButtonWithLabel("Edit")
		edit.ConnectClicked(func() { w.showPresetDetail(name) })
		actions.Attach(apply, 0, 0, 1, 1)
		actions.Attach(edit, 1, 0, 1, 1)
		focus(apply, 0)
		focus(edit, 1)
		for col, target := range []struct {
			source, label, short string
			assigned             bool
		}{{"ac", "plugged in", "AC", ac}, {"battery", "on battery", "Battery", battery}} {
			btn := gtk.NewButtonWithLabel(target.short)
			btn.AddCSSClass("assign-toggle")
			toggleClass(btn, "assigned", target.assigned)
			if target.assigned {
				btn.SetTooltipText(fmt.Sprintf("%s is the %s preset", name, target.label))
			} else {
				btn.SetTooltipText("Use " + name + " " + target.label)
			}
			btn.ConnectClicked(func() {
				if target.assigned {
					w.showPresetFeedback(fmt.Sprintf("%s is already the %s preset. Assign another preset to replace it.", name, target.label))
					return
				}
				message := fmt.Sprintf("%s set for %s", name, target.label)
				if enabled && w.state.PowerSource == target.source {
					message += " · applied"
				}
				if target.source == "ac" {
					w.setPresetPolicy(enabled, name, "", message)
				} else {
					w.setPresetPolicy(enabled, "", name, message)
				}
			})
			actions.Attach(btn, 2+col, 0, 1, 1)
			focus(btn, 2+col)
		}
		card.Append(actions)
		row++
		w.presetsList.Append(card)
	}
	focus(w.presetSaveBtn, 0)
	row++
	focus(w.presetRestoreBtn, 0)
}

func pill(text, class string) *gtk.Label {
	label := gtk.NewLabel(text)
	label.AddCSSClass("pill")
	if class != "" {
		label.AddCSSClass(class)
	}
	label.SetVAlign(gtk.AlignCenter)
	return label
}

// autoPresetName names a snapshot of the current settings, e.g.
// "Balanced 70 W", adding a number when the name is taken.
func autoPresetName(state *api.State) string {
	base := "Preset"
	if state != nil && state.Profile != "" {
		base = strings.Title(state.Profile) //nolint:staticcheck // ASCII daemon value
	}
	if state != nil && state.TDPActive && state.TDP != nil {
		base += fmt.Sprintf(" %d W", state.TDP.PL1SPL)
	}
	name := base
	for i := 2; state != nil && hasPreset(state, name); i++ {
		name = fmt.Sprintf("%s %d", base, i)
	}
	return name
}

func hasPreset(state *api.State, name string) bool {
	_, ok := state.Presets[name]
	return ok
}

func (w *Window) buildPresetDetailView() *gtk.Box {
	view := gtk.NewBox(gtk.OrientationVertical, 0)
	header, back, title := viewHeader("Preset", func() { w.leavePresetDetail() })
	w.presetDetailBackBtn = back
	w.presetDetailTitle = title
	view.Append(header)
	w.presetDetailContent = gtk.NewBox(gtk.OrientationVertical, 8)
	w.presetDetailContent.SetMarginTop(4)
	w.presetDetailContent.SetMarginBottom(12)
	w.presetDetailContent.SetMarginStart(12)
	w.presetDetailContent.SetMarginEnd(12)
	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(w.presetDetailContent)
	w.presetDetailScroll = scroll
	view.Append(scroll)
	return view
}

func (w *Window) showPresetDetail(name string) {
	if w.viewStack == nil || w.state == nil {
		return
	}
	if w.presetDetailContent == nil {
		w.viewStack.AddNamed(w.buildPresetDetailView(), "preset-detail")
	}
	w.presetDetailName = name
	w.rebuildPresetDetail()
	w.viewStack.SetVisibleChildName("preset-detail")
	w.swapFocusList(w.presetDetailFocusItems)
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

// leavePresetDetail returns to the Presets screen, asking first when the
// editor holds unsaved changes.
func (w *Window) leavePresetDetail() {
	if w.presetEditorDirty == nil || !w.presetEditorDirty() {
		w.showPresetsView()
		return
	}
	w.showConfirmation("Discard Changes", fmt.Sprintf("Discard unsaved changes to %q?", w.presetDetailName), "Discard", "preset-detail", true, func() {
		w.showPresetsView()
	})
}

func (w *Window) returnFromConfirmation() {
	returnView := w.confirmReturnView
	w.confirmAction = nil
	w.confirmReturnView = ""
	switch returnView {
	case "preset-detail":
		w.viewStack.SetVisibleChildName("preset-detail")
		w.swapFocusList(w.presetDetailFocusItems)
	case "custom":
		w.viewStack.SetVisibleChildName("custom")
		w.swapFocusList(w.customFocusItems)
	default:
		w.showPresetsView()
	}
}

func sortedPresetNames(presets map[string]api.Preset) []string {
	return slices.Sorted(maps.Keys(presets))
}

// assignedForSource returns the preset assigned to the current power source,
// whether or not automation is enabled.
func assignedForSource(state *api.State) string {
	if state == nil || state.PowerPolicy == nil {
		return ""
	}
	switch state.PowerSource {
	case "ac":
		return state.PowerPolicy.ACPreset
	case "battery":
		return state.PowerPolicy.BatteryPreset
	}
	return ""
}

// automationCurrent returns the preset automation wants right now and
// whether a manual change has overridden it until the next plug change.
func automationCurrent(state *api.State) (name string, manual bool) {
	if !currentPolicyEnabled(state) {
		return "", false
	}
	name = assignedForSource(state)
	return name, name != "" && state.ActivePreset != name
}

func setAssignmentPill(pill *gtk.Label, active, manual bool) {
	pill.SetVisible(active)
	if manual {
		pill.SetLabel("Manual")
		pill.AddCSSClass("warning")
	} else {
		pill.SetLabel("Active")
		pill.RemoveCSSClass("warning")
	}
}

func presetDetailLine(state *api.State, name string) string {
	if state == nil {
		return ""
	}
	preset, ok := state.Presets[name]
	if !ok {
		return ""
	}
	return "applies " + compactPresetSummary(preset)
}

func currentPolicyEnabled(state *api.State) bool {
	return state != nil && state.PowerPolicy != nil && state.PowerPolicy.Enabled
}

func powerPolicyAssignments(state *api.State) (ac, battery string) {
	ac, battery = "Not assigned", "Not assigned"
	if state == nil || state.PowerPolicy == nil {
		return ac, battery
	}
	if state.PowerPolicy.ACPreset != "" {
		ac = state.PowerPolicy.ACPreset
	}
	if state.PowerPolicy.BatteryPreset != "" {
		battery = state.PowerPolicy.BatteryPreset
	}
	return ac, battery
}

func presetUsage(name string, state *api.State) string {
	if state == nil {
		return "Not assigned"
	}
	var uses []string
	if state.ActivePreset == name {
		uses = append(uses, "Active now")
	}
	if state.PowerPolicy != nil {
		if state.PowerPolicy.ACPreset == name {
			uses = append(uses, "Plugged in")
		}
		if state.PowerPolicy.BatteryPreset == name {
			uses = append(uses, "On battery")
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

// compactPresetSummary lists only what the preset changes, e.g.
// "Balanced · 70 W · UV -30 · 60 Hz".
func compactPresetSummary(preset api.Preset) string {
	profile := strings.Title(preset.Profile) //nolint:staticcheck // ASCII daemon value
	if profile == "" {
		profile = "Default"
	}
	parts := []string{profile}
	if preset.TDP != nil {
		parts = append(parts, fmt.Sprintf("%d W", preset.TDP.PL1SPL))
	}
	if preset.Undervolt != nil {
		parts = append(parts, fmt.Sprintf("UV %d", preset.Undervolt.CPUCO))
	}
	if preset.RefreshRate > 0 {
		parts = append(parts, fmt.Sprintf("%d Hz", preset.RefreshRate))
	}
	if presetHighTDP(preset) {
		parts = append(parts, "safety fans")
	}
	return strings.Join(parts, " · ")
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

func (w *Window) setPresetPolicy(enabled bool, ac, battery, success string) {
	if w.state == nil {
		return
	}
	w.queueStateAction(stateAction{
		name:         "set power automation",
		reportStatus: true,
		fn: func() (bool, error) {
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
		},
		onApplied: func() {
			if success != "" {
				w.showPresetFeedback(success)
			}
		},
	})
}

type stateAction struct {
	name         string
	fn           func() (bool, error)
	reportStatus bool // progress + "Applied"; failures are always reported
	onApplied    func()
	onFailed     func(error)
}

func (w *Window) runStateAction(action string, fn func() (bool, error)) {
	w.queueStateAction(stateAction{name: action, fn: fn, reportStatus: true})
}

// runStateActionQuiet is for controls driven by a drag or a stream of taps
// (brightness, speed, scroll level): no progress line and no "Applied" flash, but a
// failure is still reported on the status line.
func (w *Window) runStateActionQuiet(action string, fn func() (bool, error), onApplied func(), onFailed func(error)) {
	w.queueStateAction(stateAction{name: action, fn: fn, onApplied: onApplied, onFailed: onFailed})
}

// runStateActionApplied confirms like runStateAction ("Applied" on the status line)
// while keeping the caller's optimistic update. For single-tap controls; controls
// driven by a drag (brightness, speed, scroll level) stay quiet so they do not flash
// on every tick. Failures are reported for every action, quiet ones included.
func (w *Window) runStateActionApplied(action string, fn func() (bool, error), onApplied func(), onFailed func(error)) {
	w.queueStateAction(stateAction{name: action, fn: fn, reportStatus: true, onApplied: onApplied, onFailed: onFailed})
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
		// Only report progress if the write is actually slow. A fast local
		// round-trip would otherwise flash a line and shove every card below
		// the automation card down mid-tap.
		w.progressGen++
		gen := w.progressGen
		glib.TimeoutAdd(300, func() bool {
			if w.progressGen == gen && w.stateActionBusy {
				w.setPresetStatus("Working...")
			}
			return false
		})
	}
	go func() {
		handled, err := action.fn()
		if err == nil && !handled {
			err = fmt.Errorf("z13ctl-plus daemon is not running")
		}
		if err != nil {
			slog.Warn(action.name+" failed", "err", err)
			glib.IdleAdd(func() {
				if action.onFailed != nil {
					action.onFailed(err) // local rollback
				}
				// Every action says why nothing changed, quiet ones included: a silent
				// failure looks exactly like a control that is not wired up. The text
				// stays until the next action so it can be read and acted on.
				w.setPresetStatus("Couldn't " + action.name + ": " + err.Error())
				if len(w.stateActionQueue) == 0 {
					w.syncState()
				}
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
					action.onApplied() // local optimistic value
				}
				w.setPresetStatus("Applied, but failed to refresh state")
				w.finishStateAction()
				return
			}
			if action.reportStatus {
				// Confirm the write landed: the control already shows its new value,
				// so a bare acknowledgement is enough to close the loop.
				w.flashStatus("Applied", 1200)
			}
			w.state = state
			// Queued actions captured newer widget values; syncing now would show
			// stale values and let the next tap send them. The last action syncs.
			if len(w.stateActionQueue) == 0 {
				w.syncState()
			}
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

// setPresetStatus writes the drawer's single feedback line. It is reserved
// space, so writing never reflows the cards.
func (w *Window) setPresetStatus(message string) {
	w.presetFeedbackGen++
	if w.statusLine != nil {
		w.statusLine.SetLabel(message)
	}
}

// flashStatus shows a message for a moment, then clears it if nothing newer
// has been written in the meantime.
func (w *Window) flashStatus(message string, ms uint) {
	w.setPresetStatus(message)
	gen := w.presetFeedbackGen
	glib.TimeoutAdd(ms, func() bool {
		if w.presetFeedbackGen == gen {
			w.setPresetStatus("")
		}
		return false
	})
}

func (w *Window) showPresetFeedback(message string) {
	w.flashStatus(message, 3000)
}
