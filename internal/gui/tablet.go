package gui

import (
	"fmt"
	"time"

	"github.com/dahui/z13ctl/api"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

const (
	tabletHeartbeatTimeout   = 75 * time.Second
	defaultScrollSensitivity = 3
	defaultScrollSpeed       = 2
)

type tabletPresentation struct {
	visible   bool
	warning   bool
	posture   string
	reason    string
	heartbeat string
}

func presentTabletStatus(status *api.TabletIntegration, now time.Time) tabletPresentation {
	if status == nil || status.LastSeen == 0 {
		return tabletPresentation{}
	}

	age := now.Unix() - status.LastSeen
	if age < 0 {
		age = 0
	}
	presentation := tabletPresentation{
		visible:   true,
		posture:   tabletPostureLabel(status.Posture),
		heartbeat: tabletHeartbeatLabel(age),
	}
	switch {
	case status.Error != "":
		presentation.warning = true
		presentation.reason = status.Error
	case !status.Healthy:
		presentation.warning = true
		presentation.reason = "Integration reported unhealthy"
	case time.Duration(age)*time.Second > tabletHeartbeatTimeout:
		presentation.warning = true
		presentation.reason = fmt.Sprintf("Heartbeat is %d seconds old", age)
	}
	return presentation
}

func tabletPostureLabel(posture string) string {
	switch posture {
	case "desktop":
		return "Desktop"
	case "laptop":
		return "Laptop"
	case "tablet":
		return "Tablet"
	default:
		return posture
	}
}

func tabletHeartbeatLabel(age int64) string {
	switch age {
	case 0:
		return "Heartbeat just now"
	case 1:
		return "Heartbeat 1 second ago"
	default:
		return fmt.Sprintf("Heartbeat %d seconds ago", age)
	}
}

func (w *Window) buildTabletChip() *gtk.ToggleButton {
	w.tabletChipLabel = gtk.NewLabel("Tablet")
	w.tabletChipLabel.SetHAlign(gtk.AlignCenter)

	chip := gtk.NewToggleButton()
	chip.SetChild(w.tabletChipLabel)
	chip.AddCSSClass("pill")
	chip.AddCSSClass("tablet-chip")
	chip.SetTooltipText("Tablet integration")
	chip.SetVisible(false)
	chip.ConnectToggled(func() {
		if w.tabletPopover != nil {
			w.tabletPopover.SetVisible(chip.Visible() && chip.Active())
		}
	})
	w.tabletChip = chip
	return chip
}

func (w *Window) buildTabletPopover() *gtk.Box {
	panel := gtk.NewBox(gtk.OrientationVertical, 6)
	panel.AddCSSClass("card")
	panel.AddCSSClass("tablet-popover")
	panel.SetMarginBottom(6)
	panel.SetMarginStart(12)
	panel.SetMarginEnd(12)
	panel.SetVisible(false)
	w.tabletPopover = panel

	statusTitle := gtk.NewLabel("TABLET KIT")
	statusTitle.SetHAlign(gtk.AlignStart)
	statusTitle.AddCSSClass("card-title")
	panel.Append(statusTitle)

	var row *gtk.Box
	row, w.tabletPostureLabel = tabletDetailRow("Posture")
	panel.Append(row)
	row, w.tabletHealthLabel = tabletDetailRow("Health")
	w.tabletHealthLabel.SetWrap(true)
	panel.Append(row)
	row, w.tabletFolioLabel = tabletDetailRow("Keyboard folio")
	panel.Append(row)
	row, w.tabletTouchscreenLabel = tabletDetailRow("Touchscreen")
	panel.Append(row)
	row, w.tabletTouchScrollLabel = tabletDetailRow("Touch scrolling")
	panel.Append(row)
	w.tabletTouchpadStatusRow, w.tabletTouchpadStatus = tabletDetailRow("Touchpad")
	panel.Append(w.tabletTouchpadStatusRow)

	panel.Append(gtk.NewSeparator(gtk.OrientationHorizontal))
	panel.Append(w.buildToggle(
		"Disable touchscreen on desktop",
		"Automatically disable the touchscreen while the Z13 is in desktop posture",
		&w.tabletDesktopSwitch,
		func(disabled bool) {
			w.changeTabletSettings(func(settings *api.TabletSettings) {
				settings.DisableTouchscreenInDesktop = disabled
			})
		},
	))
	panel.Append(w.buildToggle(
		"Two-finger hold context menu",
		"Hold two stationary fingers to open the context menu; moving fingers scrolls and cancels",
		&w.tabletTwoFingerHoldSwitch,
		func(enabled bool) {
			w.changeTabletSettings(func(settings *api.TabletSettings) {
				settings.TwoFingerHoldContextMenu = enabled
			})
		},
	))
	twoFingerDetail := gtk.NewLabel("Holding two stationary fingers about 650 ms opens the context menu. Moving fingers scrolls and cancels.")
	twoFingerDetail.SetHAlign(gtk.AlignStart)
	twoFingerDetail.SetWrap(true)
	twoFingerDetail.SetMaxWidthChars(30)
	twoFingerDetail.AddCSSClass("setting-description")
	panel.Append(twoFingerDetail)

	w.tabletTouchpadRow = w.buildTabletTouchpadToggle()
	panel.Append(w.tabletTouchpadRow)
	panel.Append(w.buildTabletTouchpadConfirmation())

	panel.Append(w.buildTabletLevel(
		"SCROLL SENSITIVITY",
		"How readily a touch drag begins scrolling",
		&w.tabletSensitivityScale,
		func(level int) {
			w.changeTabletSettings(func(settings *api.TabletSettings) {
				settings.ScrollSensitivity = level
			})
		},
	))

	panel.Append(w.buildTabletLevel(
		"SCROLL SPEED",
		"How far content moves during touch scrolling",
		&w.tabletSpeedScale,
		func(level int) {
			w.changeTabletSettings(func(settings *api.TabletSettings) {
				settings.ScrollSpeed = level
			})
		},
	))

	w.tabletResetBtn = gtk.NewButtonWithLabel("Reset Scroll Defaults")
	w.tabletResetBtn.AddCSSClass("action-btn")
	w.tabletResetBtn.SetHExpand(true)
	w.tabletResetBtn.SetTooltipText("Restore scroll sensitivity to 3 and scroll speed to 2")
	w.tabletResetBtn.ConnectClicked(func() {
		w.changeTabletSettings(func(settings *api.TabletSettings) {
			settings.ScrollSensitivity = defaultScrollSensitivity
			settings.ScrollSpeed = defaultScrollSpeed
		})
	})
	panel.Append(w.tabletResetBtn)

	w.tabletFeedback = gtk.NewLabel("")
	w.tabletFeedback.SetHAlign(gtk.AlignStart)
	w.tabletFeedback.SetWrap(true)
	w.tabletFeedback.AddCSSClass("card-sub")
	w.tabletFeedback.AddCSSClass("warning")
	w.tabletFeedback.SetVisible(false)
	panel.Append(w.tabletFeedback)

	return panel
}

func tabletDetailRow(name string) (*gtk.Box, *gtk.Label) {
	row := gtk.NewBox(gtk.OrientationHorizontal, 6)
	nameLabel := gtk.NewLabel(name)
	nameLabel.SetHAlign(gtk.AlignStart)
	nameLabel.SetHExpand(true)
	nameLabel.AddCSSClass("scale-name")
	value := gtk.NewLabel("")
	value.SetHAlign(gtk.AlignEnd)
	value.SetMaxWidthChars(24)
	value.AddCSSClass("card-sub")
	row.Append(nameLabel)
	row.Append(value)
	return row, value
}

func (w *Window) buildTabletTouchpadToggle() *gtk.Box {
	row := gtk.NewBox(gtk.OrientationHorizontal, 4)
	row.AddCSSClass("settings-row")
	row.SetTooltipText("Enable the keyboard folio touchpad")
	label := gtk.NewLabel("Enable folio touchpad")
	label.SetHAlign(gtk.AlignStart)
	label.SetHExpand(true)
	label.AddCSSClass("toggle-label")
	row.Append(label)

	w.tabletTouchpadSwitch = gtk.NewSwitch()
	w.tabletTouchpadSwitch.ConnectStateSet(func(enabled bool) bool {
		if w.syncing {
			return false
		}
		if enabled {
			w.showTabletTouchpadConfirmation()
			return true
		}
		w.changeTabletSettings(func(settings *api.TabletSettings) {
			settings.TouchpadEnabled = false
		})
		return false
	})
	if w.gamescope {
		addTouchActivate(w.tabletTouchpadSwitch, w.toggleTabletTouchpad)
	}
	row.Append(w.tabletTouchpadSwitch)
	return row
}

func (w *Window) buildTabletTouchpadConfirmation() *gtk.Box {
	confirmation := gtk.NewBox(gtk.OrientationVertical, 6)
	confirmation.AddCSSClass("confirm-message")
	confirmation.SetVisible(false)

	message := gtk.NewLabel("GZ302EA firmware can make the cursor jump while the folio touchpad is enabled. Enable it anyway?")
	message.SetHAlign(gtk.AlignStart)
	message.SetWrap(true)
	message.SetMaxWidthChars(30)
	confirmation.Append(message)

	actions := gtk.NewBox(gtk.OrientationHorizontal, 4)
	actions.AddCSSClass("btn-group")
	actions.SetHomogeneous(true)
	w.tabletConfirmCancel = gtk.NewButtonWithLabel("Cancel")
	w.tabletConfirmCancel.ConnectClicked(func() { confirmation.SetVisible(false) })
	w.tabletConfirmEnable = gtk.NewButtonWithLabel("Enable")
	w.tabletConfirmEnable.ConnectClicked(func() {
		confirmation.SetVisible(false)
		if w.tabletFolioPresent() {
			w.changeTabletSettings(func(settings *api.TabletSettings) {
				settings.TouchpadEnabled = true
			})
		}
	})
	actions.Append(w.tabletConfirmCancel)
	actions.Append(w.tabletConfirmEnable)
	confirmation.Append(actions)
	w.tabletTouchpadConfirm = confirmation
	return confirmation
}

func (w *Window) buildTabletLevel(title, description string, scale **gtk.Scale, changed func(int)) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 2)
	label := gtk.NewLabel(title)
	label.SetHAlign(gtk.AlignStart)
	label.AddCSSClass("scale-name")
	box.Append(label)

	sc := gtk.NewScaleWithRange(gtk.OrientationHorizontal, 1, 5, 1)
	sc.SetDigits(0)
	sc.SetDrawValue(true)
	sc.SetRoundDigits(0)
	sc.SetHExpand(true)
	sc.SetFocusable(true)
	sc.SetTooltipText(title + ", level 1 through 5")
	label.SetMnemonicWidget(sc)
	sc.ConnectValueChanged(func() {
		if !w.syncing {
			changed(int(sc.Value()))
		}
	})
	*scale = sc
	box.Append(sc)

	detail := gtk.NewLabel(description)
	detail.SetHAlign(gtk.AlignStart)
	detail.SetWrap(true)
	detail.SetMaxWidthChars(30)
	detail.AddCSSClass("setting-description")
	box.Append(detail)
	return box
}

func (w *Window) syncTablet() {
	if w.tabletChip == nil || w.tabletPopover == nil {
		return
	}
	var status *api.TabletIntegration
	if w.state != nil {
		status = w.state.TabletIntegration
	}
	presentation := presentTabletStatus(status, time.Now())
	w.tabletChip.SetVisible(presentation.visible)
	if !presentation.visible {
		w.tabletChip.SetActive(false)
		w.tabletPopover.SetVisible(false)
		w.tabletTouchpadConfirm.SetVisible(false)
		return
	}

	chipLabel := presentation.posture
	w.tabletChip.RemoveCSSClass("warning")
	if presentation.warning {
		chipLabel += " - Warning"
		w.tabletChip.AddCSSClass("warning")
		w.tabletChip.SetTooltipText(presentation.reason)
	} else {
		w.tabletChip.SetTooltipText("Tablet kit: " + presentation.posture)
	}
	w.tabletChipLabel.SetLabel(chipLabel)
	w.tabletPopover.SetVisible(w.tabletChip.Active())

	w.tabletPostureLabel.SetLabel(presentation.posture)
	w.tabletHealthLabel.RemoveCSSClass("warning")
	if presentation.warning {
		w.tabletHealthLabel.SetLabel(presentation.reason)
		w.tabletHealthLabel.SetTooltipText(presentation.reason)
		w.tabletHealthLabel.AddCSSClass("warning")
	} else {
		w.tabletHealthLabel.SetLabel("Healthy - " + presentation.heartbeat)
		w.tabletHealthLabel.SetTooltipText("")
	}
	w.tabletFolioLabel.SetLabel(enabledLabel(status.FolioPresent, "Attached", "Detached"))
	w.tabletTouchscreenLabel.SetLabel(enabledLabel(status.TouchscreenEnabled, "Enabled", "Disabled"))
	w.tabletTouchScrollLabel.SetLabel(enabledLabel(status.TouchScrollEnabled, "Enabled", "Disabled"))
	w.tabletTouchpadStatus.SetLabel(enabledLabel(status.TouchpadEnabled, "Enabled", "Disabled"))
	w.tabletTouchpadStatusRow.SetVisible(status.FolioPresent)
	w.tabletTouchpadRow.SetVisible(status.FolioPresent)
	if !status.FolioPresent {
		w.tabletTouchpadConfirm.SetVisible(false)
	}

	if w.state == nil {
		return
	}
	if w.tabletPendingSettings != nil && w.state.TabletSettings == *w.tabletPendingSettings {
		w.tabletPendingSettings = nil
	}
	settings := w.state.TabletSettings
	if w.tabletPendingSettings != nil {
		settings = *w.tabletPendingSettings
	}
	w.tabletDesktopSwitch.SetActive(settings.DisableTouchscreenInDesktop)
	w.tabletTwoFingerHoldSwitch.SetActive(settings.TwoFingerHoldContextMenu)
	w.tabletTouchpadSwitch.SetActive(settings.TouchpadEnabled)
	w.tabletSensitivityScale.SetValue(float64(settings.ScrollSensitivity))
	w.tabletSpeedScale.SetValue(float64(settings.ScrollSpeed))
}

func enabledLabel(enabled bool, yes, no string) string {
	if enabled {
		return yes
	}
	return no
}

func (w *Window) changeTabletSettings(change func(*api.TabletSettings)) {
	if w.state == nil {
		return
	}
	settings := w.state.TabletSettings
	if w.tabletPendingSettings != nil {
		settings = *w.tabletPendingSettings
	}
	before := settings
	change(&settings)
	if settings == before {
		return
	}

	payload := settings
	w.tabletPendingSettings = &payload
	w.syncing = true
	w.syncTablet()
	w.syncing = false
	w.runStateActionQuiet("tablet settings", func() (bool, error) {
		return api.SendTabletSettingsSet(payload)
	}, func() {
		w.setTabletFeedback("")
	}, func(err error) {
		if w.tabletPendingSettings != nil && *w.tabletPendingSettings == payload {
			w.tabletPendingSettings = nil
		}
		w.setTabletFeedback("Could not apply tablet settings: " + err.Error())
		w.syncing = true
		w.syncTablet()
		w.syncing = false
	})
}

func (w *Window) setTabletFeedback(message string) {
	if w.tabletFeedback == nil {
		return
	}
	w.tabletFeedback.SetLabel(message)
	w.tabletFeedback.SetVisible(message != "")
}

func (w *Window) tabletFolioPresent() bool {
	return w.state != nil && w.state.TabletIntegration != nil && w.state.TabletIntegration.FolioPresent
}

func (w *Window) showTabletTouchpadConfirmation() {
	if w.tabletTouchpadConfirm != nil && w.tabletFolioPresent() {
		w.tabletTouchpadConfirm.SetVisible(true)
	}
}

func (w *Window) toggleTabletTouchpad() {
	if w.tabletTouchpadSwitch.Active() {
		w.changeTabletSettings(func(settings *api.TabletSettings) {
			settings.TouchpadEnabled = false
		})
		return
	}
	w.showTabletTouchpadConfirmation()
}

func (w *Window) tabletScaleFocused() bool {
	return (w.tabletSensitivityScale != nil && w.tabletSensitivityScale.HasFocus()) ||
		(w.tabletSpeedScale != nil && w.tabletSpeedScale.HasFocus())
}

func (w *Window) tabletFocusItems() []focusItem {
	if w.tabletChip == nil {
		return nil
	}
	panelVisible := func() bool { return w.tabletChip.Visible() && w.tabletChip.Active() }
	touchpadVisible := func() bool { return panelVisible() && w.tabletFolioPresent() }
	confirmVisible := func() bool { return touchpadVisible() && w.tabletTouchpadConfirm.Visible() }
	sensitivityLeft, sensitivityRight, sensitivityGet, sensitivitySet := scaleAdjust(w.tabletSensitivityScale, 1)
	speedLeft, speedRight, speedGet, speedSet := scaleAdjust(w.tabletSpeedScale, 1)
	return []focusItem{
		{widget: w.tabletChip, row: -11, col: 0, section: "tablet", onActivate: func() { w.tabletChip.SetActive(!w.tabletChip.Active()) }},
		{widget: w.tabletDesktopSwitch, row: -10, col: 0, section: "tablet", isVisible: panelVisible, onActivate: func() { w.tabletDesktopSwitch.SetActive(!w.tabletDesktopSwitch.Active()) }},
		{widget: w.tabletTwoFingerHoldSwitch, row: -9, col: 0, section: "tablet", isVisible: panelVisible, onActivate: func() { w.tabletTwoFingerHoldSwitch.SetActive(!w.tabletTwoFingerHoldSwitch.Active()) }},
		{widget: w.tabletTouchpadSwitch, row: -8, col: 0, section: "tablet", isVisible: touchpadVisible, onActivate: w.toggleTabletTouchpad},
		{widget: w.tabletConfirmCancel, row: -7, col: 0, section: "tablet-confirm", isVisible: confirmVisible, onActivate: func() { w.tabletConfirmCancel.Activate() }},
		{widget: w.tabletConfirmEnable, row: -7, col: 1, section: "tablet-confirm", isVisible: confirmVisible, onActivate: func() { w.tabletConfirmEnable.Activate() }},
		{
			widget: w.tabletSensitivityScale, row: -6, col: 0, section: "tablet-sensitivity", isVisible: panelVisible, editable: true,
			onLeft: sensitivityLeft, onRight: sensitivityRight, getValue: sensitivityGet, setValue: sensitivitySet,
		},
		{
			widget: w.tabletSpeedScale, row: -5, col: 0, section: "tablet-speed", isVisible: panelVisible, editable: true,
			onLeft: speedLeft, onRight: speedRight, getValue: speedGet, setValue: speedSet,
		},
		{widget: w.tabletResetBtn, row: -4, col: 0, section: "tablet", isVisible: panelVisible, onActivate: func() { w.tabletResetBtn.Activate() }},
	}
}
