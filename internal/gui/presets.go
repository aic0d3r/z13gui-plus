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

	w.presetAssignments = gtk.NewBox(gtk.OrientationVertical, 2)
	w.presetACRow, w.presetACLabel, w.presetACStatus = automationSummaryRow("PLUGGED IN")
	w.presetBatteryRow, w.presetBatteryLabel, w.presetBatteryStatus = automationSummaryRow("ON BATTERY")
	w.presetAssignments.Append(w.presetACRow)
	w.presetAssignments.Append(w.presetBatteryRow)
	card.Append(w.presetAssignments)

	w.presetsBtn = gtk.NewButtonWithLabel("Edit Assignments")
	w.presetsBtn.AddCSSClass("action-btn")
	w.presetsBtn.ConnectClicked(func() { w.showPresetsView() })
	card.Append(w.presetsBtn)
	return card
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

func automationSummaryRow(source string) (row *gtk.Box, preset, status *gtk.Label) {
	row = gtk.NewBox(gtk.OrientationHorizontal, 6)
	row.AddCSSClass("automation-summary-row")
	sourceLabel := gtk.NewLabel(source)
	sourceLabel.SetHAlign(gtk.AlignStart)
	sourceLabel.AddCSSClass("automation-source")
	row.Append(sourceLabel)
	preset = gtk.NewLabel("Not assigned")
	preset.SetHAlign(gtk.AlignEnd)
	preset.SetHExpand(true)
	preset.AddCSSClass("preset-summary")
	row.Append(preset)
	status = gtk.NewLabel("ACTIVE")
	status.AddCSSClass("pill")
	status.SetVisible(false)
	row.Append(status)
	return row, preset, status
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
	header, back, title := viewHeader("Power Automation", func() { w.showMainView() })
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

	assignments := plainPowerCard("ASSIGNMENTS")
	w.acAssignment, w.acAssignmentLabel, w.acAssignmentStatus, w.acChangeBtn = w.buildAssignmentCard("PLUGGED IN", "ac")
	w.batteryAssignment, w.batteryAssignLabel, w.batteryAssignStatus, w.batteryChangeBtn = w.buildAssignmentCard("ON BATTERY", "battery")
	assignments.Append(w.acChangeBtn)
	assignments.Append(w.batteryChangeBtn)
	content.Append(assignments)

	w.presetAssignmentStatus = gtk.NewLabel("")
	w.presetAssignmentStatus.SetHAlign(gtk.AlignStart)
	w.presetAssignmentStatus.SetWrap(true)
	w.presetAssignmentStatus.AddCSSClass("card-sub")
	content.Append(w.presetAssignmentStatus)

	w.presetLibraryBtn = gtk.NewButtonWithLabel("Preset Library")
	w.presetLibraryBtn.AddCSSClass("action-btn")
	w.presetLibraryBtn.ConnectClicked(func() { w.showPresetLibraryView() })
	content.Append(w.presetLibraryBtn)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(content)
	w.presetsScroll = scroll
	view.Append(scroll)
	w.presetFocusItems = []focusItem{
		{widget: back, row: 0, col: 0, section: "automation-header", onActivate: func() { back.Activate() }},
		{widget: w.automationViewSw, row: 0, col: 1, section: "automation-header", onActivate: func() { w.automationViewSw.SetActive(!w.automationViewSw.Active()) }},
		{widget: w.acChangeBtn, row: 1, col: 0, section: "assignments", onActivate: func() { w.acChangeBtn.Activate() }},
		{widget: w.batteryChangeBtn, row: 2, col: 0, section: "assignments", onActivate: func() { w.batteryChangeBtn.Activate() }},
		{widget: w.presetLibraryBtn, row: 3, col: 0, section: "preset-library", onActivate: func() { w.presetLibraryBtn.Activate() }},
	}
	return view
}

func (w *Window) buildPresetLibraryView() *gtk.Box {
	view := gtk.NewBox(gtk.OrientationVertical, 0)
	header, back, _ := viewHeader("Preset Library", func() { w.showPresetsView() })
	w.presetLibraryBackBtn = back
	view.Append(header)

	content := gtk.NewBox(gtk.OrientationVertical, 8)
	content.SetMarginTop(4)
	content.SetMarginBottom(12)
	content.SetMarginStart(12)
	content.SetMarginEnd(12)

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
			w.setPresetStatus("A preset with that name already exists. Open its card to update it.")
			return
		}
		w.runStateAction("create preset", func() (bool, error) { return api.SendPresetSave(name) })
	})
	create.Append(w.presetSaveBtn)
	content.Append(create)

	w.presetRestoreBtn = gtk.NewButtonWithLabel("Restore Recommended Automation")
	w.presetRestoreBtn.AddCSSClass("action-btn")
	w.presetRestoreBtn.ConnectClicked(func() {
		w.showConfirmation(
			"Restore Recommended Automation",
			"Create or refresh Plugged In and On Battery, assign them to the correct power sources, and enable automation? Other presets and your battery charge limit are preserved.",
			"Restore",
			"preset-library",
			false,
			func() { w.runStateAction("restore recommended automation", api.SendRestoreRecommended) },
		)
	})
	content.Append(w.presetRestoreBtn)
	w.presetStatus = gtk.NewLabel("")
	w.presetStatus.SetHAlign(gtk.AlignStart)
	w.presetStatus.SetWrap(true)
	content.Append(w.presetStatus)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(content)
	w.presetLibraryScroll = scroll
	view.Append(scroll)
	w.rebuildPresetRows()
	return view
}

func (w *Window) buildAssignmentCard(title, target string) (row *gtk.Box, summary, status *gtk.Label, change *gtk.Button) {
	row = gtk.NewBox(gtk.OrientationHorizontal, 8)
	text := gtk.NewBox(gtk.OrientationVertical, 2)
	text.SetHExpand(true)
	titleLabel := gtk.NewLabel(title)
	titleLabel.SetHAlign(gtk.AlignStart)
	titleLabel.AddCSSClass("card-title")
	text.Append(titleLabel)
	summary = gtk.NewLabel("Not assigned")
	summary.SetHAlign(gtk.AlignStart)
	summary.SetWrap(true)
	summary.AddCSSClass("preset-summary")
	text.Append(summary)
	row.Append(text)
	status = gtk.NewLabel("ACTIVE")
	status.AddCSSClass("pill")
	status.SetVAlign(gtk.AlignCenter)
	status.SetVisible(false)
	row.Append(status)
	chevron := gtk.NewImageFromIconName("pan-end-symbolic")
	chevron.SetVAlign(gtk.AlignCenter)
	row.Append(chevron)
	change = gtk.NewButton()
	change.AddCSSClass("assignment-row")
	change.SetChild(row)
	change.ConnectClicked(func() { w.showPresetChooser(target) })
	return row, summary, status, change
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
	w.presetDetailName = ""
	w.syncPresets()
	w.viewStack.SetVisibleChildName("presets")
	w.swapFocusList(w.presetFocusItems)
}

func (w *Window) showPresetLibraryView() {
	if w.viewStack == nil {
		return
	}
	if w.presetLibraryScroll == nil {
		w.viewStack.AddNamed(w.buildPresetLibraryView(), "preset-library")
	}
	w.presetDetailName = ""
	w.syncPresets()
	w.viewStack.SetVisibleChildName("preset-library")
	w.swapFocusList(w.presetLibraryFocusItems)
}

func (w *Window) syncPresets() {
	if w.state == nil {
		return
	}
	enabled := currentPolicyEnabled(w.state)
	ac, battery := powerPolicyAssignments(w.state)
	if w.automationSwitch != nil {
		w.automationSwitch.SetActive(enabled)
	}
	if w.automationViewSw != nil {
		w.automationViewSw.SetActive(enabled)
	}

	acActive := enabled && w.state.PowerSource == "ac"
	batteryActive := enabled && w.state.PowerSource == "battery"
	if w.presetAssignments != nil {
		w.presetACLabel.SetLabel(ac)
		w.presetBatteryLabel.SetLabel(battery)
		if ac == "Not assigned" {
			w.presetACRow.AddCSSClass("assignment-missing")
		} else {
			w.presetACRow.RemoveCSSClass("assignment-missing")
		}
		if battery == "Not assigned" {
			w.presetBatteryRow.AddCSSClass("assignment-missing")
		} else {
			w.presetBatteryRow.RemoveCSSClass("assignment-missing")
		}
		w.presetACStatus.SetVisible(acActive)
		w.presetBatteryStatus.SetVisible(batteryActive)
		if enabled {
			w.presetAssignments.RemoveCSSClass("automation-off")
		} else {
			w.presetAssignments.AddCSSClass("automation-off")
		}
		if acActive {
			w.presetACRow.AddCSSClass("current-assignment")
		} else {
			w.presetACRow.RemoveCSSClass("current-assignment")
		}
		if batteryActive {
			w.presetBatteryRow.AddCSSClass("current-assignment")
		} else {
			w.presetBatteryRow.RemoveCSSClass("current-assignment")
		}
	}
	if w.acAssignment != nil {
		w.acAssignmentLabel.SetLabel(ac)
		w.batteryAssignLabel.SetLabel(battery)
		if ac == "Not assigned" {
			w.acChangeBtn.AddCSSClass("assignment-missing")
		} else {
			w.acChangeBtn.RemoveCSSClass("assignment-missing")
		}
		if battery == "Not assigned" {
			w.batteryChangeBtn.AddCSSClass("assignment-missing")
		} else {
			w.batteryChangeBtn.RemoveCSSClass("assignment-missing")
		}
		w.acAssignmentStatus.SetVisible(acActive)
		w.batteryAssignStatus.SetVisible(batteryActive)
		if acActive {
			w.acChangeBtn.AddCSSClass("current-assignment")
		} else {
			w.acChangeBtn.RemoveCSSClass("current-assignment")
		}
		if batteryActive {
			w.batteryChangeBtn.AddCSSClass("current-assignment")
		} else {
			w.batteryChangeBtn.RemoveCSSClass("current-assignment")
		}
	}
	if w.presetCurrent != nil {
		w.presetCurrent.SetLabel(currentSettingsSummary(w.state))
	}
	w.rebuildPresetRows()
	if w.presetDetailName != "" && w.presetDetailContent != nil {
		w.rebuildPresetDetail()
	}
	if w.viewStack != nil {
		switch w.viewStack.VisibleChildName() {
		case "presets":
			w.swapFocusList(w.presetFocusItems)
		case "preset-library":
			w.swapFocusList(w.presetLibraryFocusItems)
		case "preset-detail":
			w.swapFocusList(w.presetDetailFocusItems)
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
	w.presetLibraryFocusItems = []focusItem{{
		widget: w.presetLibraryBackBtn, row: 0, col: 0, section: "library-header",
		onActivate: func() { w.presetLibraryBackBtn.Activate() },
	}}
	row := 1
	for _, name := range sortedPresetNames(w.state.Presets) {
		preset := w.state.Presets[name]
		button := gtk.NewButton()
		button.AddCSSClass("preset-row")
		body := gtk.NewBox(gtk.OrientationVertical, 4)
		body.SetHExpand(true)
		header := gtk.NewBox(gtk.OrientationHorizontal, 6)
		title := gtk.NewLabel(name)
		title.SetHAlign(gtk.AlignStart)
		title.SetHExpand(true)
		title.AddCSSClass("card-title")
		header.Append(title)
		usage := gtk.NewLabel(presetUsage(name, w.state))
		usage.SetHAlign(gtk.AlignEnd)
		usage.AddCSSClass("card-sub")
		header.Append(usage)
		body.Append(header)
		summary := gtk.NewLabel(compactPresetSummary(preset))
		summary.SetHAlign(gtk.AlignStart)
		summary.SetWrap(true)
		summary.AddCSSClass("preset-summary")
		body.Append(summary)
		contents := gtk.NewBox(gtk.OrientationHorizontal, 8)
		contents.Append(body)
		chevron := gtk.NewImageFromIconName("pan-end-symbolic")
		chevron.SetVAlign(gtk.AlignCenter)
		contents.Append(chevron)
		button.SetChild(contents)
		button.ConnectClicked(func() { w.showPresetDetail(name) })
		w.presetsList.Append(button)
		w.presetLibraryFocusItems = append(w.presetLibraryFocusItems, focusItem{widget: button, row: row, col: 0, section: "saved-presets", onActivate: func() { button.Activate() }})
		row++
	}
	w.presetLibraryFocusItems = append(w.presetLibraryFocusItems,
		focusItem{widget: w.presetNameEntry, row: row, col: 0, section: "create-preset", onActivate: func() { w.presetNameEntry.GrabFocus() }},
		focusItem{widget: w.presetSaveBtn, row: row + 1, col: 0, section: "create-preset", onActivate: func() { w.presetSaveBtn.Activate() }},
		focusItem{widget: w.presetRestoreBtn, row: row + 2, col: 0, section: "restore", onActivate: func() { w.presetRestoreBtn.Activate() }},
	)
}

func (w *Window) buildPresetDetailView() *gtk.Box {
	view := gtk.NewBox(gtk.OrientationVertical, 0)
	header, back, title := viewHeader("Preset", func() { w.showPresetLibraryView() })
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

func (w *Window) rebuildPresetDetail() {
	preset, ok := w.state.Presets[w.presetDetailName]
	if !ok {
		w.showPresetsView()
		return
	}
	for child := w.presetDetailContent.FirstChild(); child != nil; child = w.presetDetailContent.FirstChild() {
		w.presetDetailContent.Remove(child)
	}
	w.presetDetailTitle.SetLabel(w.presetDetailName)
	usage := gtk.NewLabel(presetUsage(w.presetDetailName, w.state))
	usage.SetHAlign(gtk.AlignStart)
	usage.AddCSSClass("card-sub")
	w.presetDetailContent.Append(usage)
	settings := plainPowerCard("SETTINGS")
	summary := gtk.NewLabel(presetSummary(preset))
	summary.SetHAlign(gtk.AlignStart)
	summary.SetWrap(true)
	summary.AddCSSClass("preset-summary")
	settings.Append(summary)
	w.presetDetailContent.Append(settings)

	update := gtk.NewButtonWithLabel("Update from Current Settings")
	update.AddCSSClass("action-btn")
	name := w.presetDetailName
	update.ConnectClicked(func() {
		w.runStateAction("update preset", func() (bool, error) { return api.SendPresetSave(name) })
	})
	w.presetDetailContent.Append(update)
	remove := gtk.NewButtonWithLabel("Delete Preset")
	remove.AddCSSClass("destructive-action")
	assigned := presetAssigned(name, w.state)
	remove.SetSensitive(!assigned)
	remove.ConnectClicked(func() {
		w.showConfirmation("Delete Preset", fmt.Sprintf("Delete %q? Current hardware settings will not change.", name), "Delete", "preset-library", true, func() {
			w.runStateAction("delete preset", func() (bool, error) { return api.SendPresetDelete(name) })
		})
	})
	w.presetDetailContent.Append(remove)
	if assigned {
		hint := gtk.NewLabel("Assign another preset before deleting this one.")
		hint.SetHAlign(gtk.AlignStart)
		hint.SetWrap(true)
		hint.AddCSSClass("card-sub")
		w.presetDetailContent.Append(hint)
	}
	w.presetDetailFocusItems = []focusItem{
		{widget: w.presetDetailBackBtn, row: 0, col: 0, section: "preset-detail-header", onActivate: func() { w.presetDetailBackBtn.Activate() }},
		{widget: update, row: 1, col: 0, section: "preset-detail", onActivate: func() { update.Activate() }},
		{widget: remove, row: 2, col: 0, section: "preset-detail", onActivate: func() { remove.Activate() }},
	}
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
	source := "Plugged In"
	if target == "battery" {
		source = "On Battery"
	}
	w.chooserFocusItems = w.chooserFocusItems[:1]
	for i, name := range sortedPresetNames(w.state.Presets) {
		button := gtk.NewButtonWithLabel(name + "\n" + compactPresetSummary(w.state.Presets[name]))
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
			message := fmt.Sprintf("%s assigned to %s", name, source)
			if appliesNow {
				message += " and applied"
			}
			if target == "ac" {
				w.setPresetPolicy(currentPolicyEnabled(w.state), name, "", message)
			} else {
				w.setPresetPolicy(currentPolicyEnabled(w.state), "", name, message)
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
	switch returnView {
	case "custom":
		w.viewStack.SetVisibleChildName("custom")
		w.swapFocusList(w.customFocusItems)
	case "preset-library":
		w.showPresetLibraryView()
	default:
		w.showPresetsView()
	}
}

func sortedPresetNames(presets map[string]api.Preset) []string {
	return slices.Sorted(maps.Keys(presets))
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
		uses = append(uses, "ACTIVE NOW")
	}
	if state.PowerPolicy != nil {
		if state.PowerPolicy.ACPreset == name {
			uses = append(uses, "PLUGGED IN")
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

func compactPresetSummary(preset api.Preset) string {
	rows := strings.Split(presetSummary(preset), "\n")
	for i := 0; i < 3; i++ {
		if _, value, ok := strings.Cut(rows[i], ": "); ok {
			rows[i] = value
		}
	}
	return strings.Join(rows[:3], " · ")
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
			err = fmt.Errorf("z13ctl-plus daemon is not running")
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
	if w.presetAssignmentStatus != nil {
		w.presetAssignmentStatus.SetLabel(message)
	}
}

func (w *Window) showPresetFeedback(message string) {
	w.setPresetStatus(message)
	glib.TimeoutAdd(3000, func() bool {
		if w.presetAssignmentStatus != nil && w.presetAssignmentStatus.Label() == message {
			w.setPresetStatus("")
		}
		return false
	})
}
