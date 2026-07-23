package gui

// controls.go — builds the entire drawer widget tree, theme picker view,
// and HSL color picker view. All views live in a gtk.Stack (both KDE and
// gamescope modes) for consistent gamepad navigation.

import (
	"fmt"
	"strings"

	"github.com/dahui/z13ctl/api"
	"github.com/dahui/z13gui/internal/theme"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// addTouchActivate works around a GTK4 X11 issue where CheckButton and Switch
// widgets don't receive touch events properly in gamescope/XWayland (their
// internal GestureClick uses BUBBLE phase, which fails for touch). Adding a
// CAPTURE-phase, touch-only gesture ensures touch taps activate these widgets.
// Mouse input is unaffected (SetTouchOnly).
func addTouchActivate(widget gtk.Widgetter, onTap func()) {
	gesture := gtk.NewGestureClick()
	gesture.SetTouchOnly(true)
	gesture.SetPropagationPhase(gtk.PhaseCapture)
	gesture.ConnectReleased(func(_ int, _, _ float64) {
		onTap()
	})
	gtk.BaseWidget(widget).AddController(gesture)
}

// buildContent builds the scrolled content box and returns it as the window child.
// Content, theme view, and color picker view are in a gtk.Stack so views can be
// swapped for gamepad navigation (and in gamescope where popovers don't work).
func (w *Window) buildContent() gtk.Widgetter {
	outer := gtk.NewBox(gtk.OrientationVertical, 0)
	outer.AddCSSClass("drawer")

	// Fixed title row — sits above the tab row, always visible.
	titleRow := gtk.NewBox(gtk.OrientationHorizontal, 0)
	titleRow.SetMarginTop(6)
	titleRow.SetMarginBottom(4)
	titleRow.SetMarginStart(14)
	titleRow.SetMarginEnd(14)

	titleLabel := gtk.NewLabel("z13ctl")
	titleLabel.SetHAlign(gtk.AlignStart)
	titleLabel.AddCSSClass("drawer-title")
	titleRow.Append(titleLabel)

	// Main tab row — Overview / Power / RGB / System. Active tab persists across drawer opens.
	tabRow := w.buildMainTabRow()

	// Tab content stack — each tab has its own scroll area.
	w.tabStack = gtk.NewStack()
	w.tabStack.SetTransitionType(gtk.StackTransitionTypeNone)
	w.tabStack.SetVExpand(true)
	w.tabStack.AddNamed(w.buildOverviewTab(), "overview")
	w.tabStack.AddNamed(w.buildPowerTab(), "power")
	w.tabStack.AddNamed(w.buildRGBTab(), "rgb")
	w.tabStack.AddNamed(w.buildSystemTab(), "system")
	w.tabStack.SetVisibleChildName(w.activeTab)
	setActiveButton(w.mainTabBtns, w.activeTab)

	mainPage := gtk.NewBox(gtk.OrientationVertical, 0)
	mainPage.Append(titleRow)
	mainPage.Append(tabRow)
	mainPage.Append(w.tabStack)

	// Stack with main, custom, theme, and color views.
	w.viewStack = gtk.NewStack()
	w.viewStack.SetTransitionType(gtk.StackTransitionTypeNone)
	w.viewStack.SetVExpand(true)
	w.viewStack.AddNamed(mainPage, "main")
	// Theme and color views are lazy-loaded on first navigation
	// to keep the initial widget tree small for fast animation.
	w.viewStack.SetVisibleChildName("main")
	outer.Append(w.viewStack)

	w.buildMainFocusList()
	w.focusItems = w.mainFocusItems

	return outer
}

// buildMainTabRow creates the top-level Power / RGB / System tab buttons.
func (w *Window) buildMainTabRow() *gtk.Box {
	row := gtk.NewBox(gtk.OrientationHorizontal, 4)
	row.SetMarginStart(12)
	row.SetMarginEnd(12)
	row.SetMarginBottom(4)
	row.AddCSSClass("main-tab-row")

	tabs := []struct{ name, label string }{
		{"overview", "Overview"},
		{"power", "Power"},
		{"rgb", "RGB"},
		{"system", "System"},
	}
	for _, t := range tabs {
		t := t
		btn := gtk.NewButtonWithLabel(t.label)
		btn.SetHExpand(true)
		btn.ConnectClicked(func() { w.switchTab(t.name) })
		if w.gamescope {
			addTouchActivate(btn, func() { btn.Activate() })
		}
		w.mainTabBtns[t.name] = btn
		row.Append(btn)
	}
	return row
}

// switchTab changes the visible tab content and rebuilds the focus list.
func (w *Window) switchTab(name string) {
	if w.tabStack == nil {
		return
	}
	w.activeTab = name
	w.tabStack.SetVisibleChildName(name)
	setActiveButton(w.mainTabBtns, name)
	w.buildMainFocusList()
	w.swapFocusList(w.mainFocusItems)
}

// buildPowerTab builds the Power tab: profile, fan preset, battery.
// Telemetry lives on the Overview tab; this tab is controls-only.
// Common controls stay visible; advanced CPU controls appear only for Custom.
func (w *Window) buildPowerTab() *gtk.ScrolledWindow {
	inner := gtk.NewBox(gtk.OrientationVertical, 6)
	inner.SetMarginTop(2)
	inner.SetMarginBottom(4)
	inner.SetMarginStart(12)
	inner.SetMarginEnd(12)

	inner.Append(groupLabel("POWER"))
	inner.Append(w.buildProfileSection())
	inner.Append(w.buildFanPresetSection())
	inner.Append(w.buildBatterySection())
	inner.Append(w.buildPresetEntrySection())
	inner.Append(w.buildCPUPowerSection())

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(inner)
	w.powerScroll = scroll
	return scroll
}

// buildRGBTab builds the RGB tab: device tabs, mode, colors, speed, brightness.
func (w *Window) buildRGBTab() *gtk.ScrolledWindow {
	inner := gtk.NewBox(gtk.OrientationVertical, 8)
	inner.SetMarginTop(4)
	inner.SetMarginBottom(12)
	inner.SetMarginStart(12)
	inner.SetMarginEnd(12)

	inner.Append(groupLabel("RGB"))
	inner.Append(sectionLabel("DEVICE"))
	inner.Append(w.buildTabRow())
	inner.Append(w.buildToggle("Lighting", "Turn lighting on or off for the selected device", &w.lightingSwitch, w.setLightingEnabled))

	controls := gtk.NewBox(gtk.OrientationVertical, 8)
	controls.Append(w.buildModeSection())

	// Initialize color inputs here so syncModeVis can reference them.
	w.color1 = w.newColorInput("FF0000", "color1-swatch", "COLOR 1")
	w.color2 = w.newColorInput("000000", "color2-swatch", "COLOR 2")
	w.updateSwatches()

	w.color1Box = colorSubBox("COLOR 1", w.color1.row)
	w.color2Box = colorSubBox("COLOR 2", w.color2.row)
	controls.Append(w.color1Box)
	controls.Append(w.color2Box)

	w.speedBox = w.buildSpeedBox()
	controls.Append(w.speedBox)
	w.brightBox = w.buildBrightnessBox()
	controls.Append(w.brightBox)
	w.rgbControlsBox = controls
	inner.Append(controls)

	// Set initial visibility based on default mode (static).
	w.syncModeVis()

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(inner)
	w.rgbScroll = scroll
	return scroll
}

// buildSystemTab builds the System tab: panel overdrive and boot sound.
func (w *Window) buildSystemTab() *gtk.ScrolledWindow {
	inner := gtk.NewBox(gtk.OrientationVertical, 8)
	inner.SetMarginTop(4)
	inner.SetMarginBottom(12)
	inner.SetMarginStart(12)
	inner.SetMarginEnd(12)

	inner.Append(groupLabel("SYSTEM"))
	display := gtk.NewBox(gtk.OrientationVertical, 6)
	display.AddCSSClass("card")
	display.Append(sectionLabel("DISPLAY"))
	display.Append(w.buildRefreshRateSection())
	display.Append(w.buildToggle("Panel Overdrive", "Enable panel overdrive for faster pixel response (may increase power use)", &w.overdriveSwitch, func(active bool) {
		v := 0
		if active {
			v = 1
		}
		w.sendOverdriveSet(v)
	}))
	inner.Append(display)

	device := gtk.NewBox(gtk.OrientationVertical, 6)
	device.AddCSSClass("card")
	device.Append(sectionLabel("DEVICE"))
	device.Append(w.buildToggle("Boot Sound", "Play startup sound when the laptop powers on", &w.bootSoundSwitch, func(active bool) {
		v := 0
		if active {
			v = 1
		}
		w.sendBootSoundSet(v)
	}))
	inner.Append(device)
	inner.Append(w.buildAppearanceSection())

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(inner)
	w.systemScroll = scroll
	return scroll
}

// buildAppearanceSection keeps theme selection with the other system settings.
func (w *Window) buildAppearanceSection() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 4)
	box.AddCSSClass("card")
	box.AddCSSClass("btn-group")
	box.Append(sectionLabel("APPEARANCE"))

	cfg := theme.LoadAppConfig()
	w.paletteBtn = gtk.NewButtonWithLabel(themeButtonLabel(cfg.Theme, cfg.Accent, w.isCustomTheme))
	w.paletteBtn.SetTooltipText("Choose theme")
	w.paletteBtn.ConnectClicked(func() { w.showThemeView() })
	box.Append(w.paletteBtn)

	return box
}

func themeButtonLabel(themeID, accentID string, custom bool) string {
	if custom {
		return "Custom Theme"
	}
	name := "ROG Dark"
	for _, builtin := range theme.Builtins {
		if builtin.ID != themeID {
			continue
		}
		name = builtin.Name
		for _, accent := range builtin.Accents {
			if accent.ID == accentID {
				return name + " / " + accent.Name
			}
		}
		break
	}
	return name
}

// buildToggle creates a compact label + switch pair for the bottom bar.
func (w *Window) buildToggle(label, tooltip string, sw **gtk.Switch, onChange func(bool)) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	box.AddCSSClass("settings-row")
	box.SetTooltipText(tooltip)
	lbl := gtk.NewLabel(label)
	lbl.AddCSSClass("toggle-label")
	lbl.SetHAlign(gtk.AlignStart)
	lbl.SetHExpand(true)
	s := gtk.NewSwitch()
	s.ConnectStateSet(func(state bool) bool {
		if !w.syncing {
			onChange(state)
		}
		return false
	})
	if w.gamescope {
		addTouchActivate(s, func() { s.SetActive(!s.Active()) })
	}
	*sw = s
	box.Append(lbl)
	box.Append(s)
	return box
}

// buildThemeView builds the theme picker as a full scrollable view.
func (w *Window) buildThemeView() *gtk.Box {
	view := gtk.NewBox(gtk.OrientationVertical, 0)

	w.themeBackBtn = gtk.NewButton()
	w.themeBackBtn.SetIconName("go-previous-symbolic")
	w.themeBackBtn.AddCSSClass("view-back-btn")
	w.themeBackBtn.ConnectClicked(func() { w.showMainView() })

	header := gtk.NewBox(gtk.OrientationHorizontal, 8)
	header.SetMarginTop(10)
	header.SetMarginBottom(6)
	header.SetMarginStart(14)
	header.Append(w.themeBackBtn)
	lbl := gtk.NewLabel("Theme")
	lbl.SetHAlign(gtk.AlignStart)
	lbl.AddCSSClass("drawer-title")
	header.Append(lbl)
	view.Append(header)

	content := gtk.NewBox(gtk.OrientationVertical, 2)
	content.SetMarginTop(4)
	content.SetMarginBottom(12)
	content.SetMarginStart(12)
	content.SetMarginEnd(12)
	w.appendThemeChoices(content)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(content)
	w.themeScroll = scroll
	view.Append(scroll)
	return view
}

// appendThemeChoices appends the theme radio buttons and accent dots to box.
// Also collects widget references in w.themeRadios and w.themeDots for
// building the theme focus list.
func (w *Window) appendThemeChoices(box *gtk.Box) {
	w.themeRadios = nil
	w.themeDots = nil

	activeCfg := theme.LoadAppConfig()
	var first *gtk.CheckButton
	for _, t := range theme.Builtins {
		id := t.ID
		btn := gtk.NewCheckButtonWithLabel(t.Name)
		if first == nil {
			first = btn
		} else {
			btn.SetGroup(first)
		}
		if id == activeCfg.Theme {
			btn.SetActive(true)
		}
		btn.ConnectToggled(func() {
			if btn.Active() {
				w.applyTheme(id, "")
			}
		})
		if w.gamescope {
			addTouchActivate(btn, func() { btn.SetActive(true) })
		}
		w.themeRadios = append(w.themeRadios, btn)
		box.Append(btn)

		dots := w.appendAccentDots(box, t.Accents,
			func(ac theme.Accent) bool { return id == activeCfg.Theme && ac.ID == activeCfg.Accent },
			func(ac theme.Accent) { btn.SetActive(true); w.applyTheme(id, ac.ID) },
		)
		w.themeDots = append(w.themeDots, dots)
	}

	// Custom theme entry — shown when theme.toml is active.
	if w.isCustomTheme {
		customBtn := gtk.NewCheckButtonWithLabel("Custom")
		if first != nil {
			customBtn.SetGroup(first)
		}
		customBtn.SetActive(true)
		customBtn.ConnectToggled(func() {
			if customBtn.Active() {
				w.applyCustomAccent("")
			}
		})
		if w.gamescope {
			addTouchActivate(customBtn, func() { customBtn.SetActive(true) })
		}
		w.themeRadios = append(w.themeRadios, customBtn)
		box.Append(customBtn)

		dots := w.appendAccentDots(box, w.customAccents,
			func(ac theme.Accent) bool { return ac.ID == activeCfg.Accent },
			func(ac theme.Accent) { w.applyCustomAccent(ac.ID) },
		)
		w.themeDots = append(w.themeDots, dots)
	}
}

// appendAccentDots builds the "Accent Color" label and dot button grid for the
// given accent list and appends both to box. Returns the dot buttons for use
// in the focus list. isActive reports whether a dot should be marked active;
// onClick is called when a dot is clicked.
func (w *Window) appendAccentDots(box *gtk.Box, accents []theme.Accent, isActive func(theme.Accent) bool, onClick func(theme.Accent)) []*gtk.Button {
	if len(accents) == 0 {
		return nil
	}
	accentLabel := gtk.NewLabel("Accent Color")
	accentLabel.SetXAlign(0)
	accentLabel.AddCSSClass("accent-label")
	accentLabel.SetMarginStart(12)
	accentLabel.SetMarginTop(2)
	box.Append(accentLabel)

	dotsGrid := gtk.NewBox(gtk.OrientationVertical, 4)
	dotsGrid.SetMarginStart(12)
	dotsGrid.SetMarginBottom(4)

	var dots []*gtk.Button
	var row *gtk.Box
	for i, ac := range accents {
		ac := ac
		if i%dotsPerRow == 0 {
			row = gtk.NewBox(gtk.OrientationHorizontal, 4)
			dotsGrid.Append(row)
		}
		dot := gtk.NewButton()
		dot.AddCSSClass("color-preset")
		dot.SetHExpand(true)
		if isActive(ac) {
			dot.AddCSSClass("accent-dot-active")
		}
		provider := gtk.NewCSSProvider()
		provider.LoadFromString("button.color-preset { background: " + ac.Hex + "; }")
		dot.StyleContext().AddProvider(provider, gtk.STYLE_PROVIDER_PRIORITY_USER+20) //nolint:staticcheck // per-widget dynamic color; no style-class alternative for unique hex backgrounds
		dot.SetTooltipText(ac.Name)
		dot.ConnectClicked(func() { onClick(ac) })
		dots = append(dots, dot)
		row.Append(dot)
	}
	box.Append(dotsGrid)
	return dots
}

// buildColorPickerView builds the HSL color picker view.
// Contains preset buttons, hue/saturation/lightness sliders, and a preview swatch.
func (w *Window) buildColorPickerView() *gtk.Box {
	view := gtk.NewBox(gtk.OrientationVertical, 8)
	view.SetMarginStart(12)
	view.SetMarginEnd(12)

	// Header: back button + dynamic title.
	w.colorViewTitle = gtk.NewLabel("COLOR")
	w.colorBackBtn = gtk.NewButton()
	w.colorBackBtn.SetIconName("go-previous-symbolic")
	w.colorBackBtn.AddCSSClass("view-back-btn")
	w.colorBackBtn.ConnectClicked(func() { w.showMainView() })

	header := gtk.NewBox(gtk.OrientationHorizontal, 8)
	header.SetMarginTop(10)
	header.SetMarginBottom(6)
	header.Append(w.colorBackBtn)
	w.colorViewTitle.SetHAlign(gtk.AlignStart)
	w.colorViewTitle.AddCSSClass("drawer-title")
	header.Append(w.colorViewTitle)
	view.Append(header)

	// 8 preset buttons.
	w.colorPickerPresets = nil
	presetsRow := gtk.NewBox(gtk.OrientationHorizontal, 4)
	for _, hex := range presetColors {
		h := hex
		btn := gtk.NewButton()
		btn.AddCSSClass("color-preset")
		btn.SetHExpand(true)
		p := gtk.NewCSSProvider()
		p.LoadFromString(fmt.Sprintf("button.color-preset { background: #%s; }", h))
		btn.StyleContext().AddProvider(p, gtk.STYLE_PROVIDER_PRIORITY_USER+5) //nolint:staticcheck // per-widget dynamic color
		btn.ConnectClicked(func() { w.colorPickerPresetClicked(h) })
		w.colorPickerPresets = append(w.colorPickerPresets, btn)
		presetsRow.Append(btn)
	}
	view.Append(presetsRow)

	// HSL sliders.
	w.colorHue = w.buildHSLScale("HUE", 0, 360)
	w.colorSat = w.buildHSLScale("SATURATION", 0, 100)
	w.colorLit = w.buildHSLScale("LIGHTNESS", 0, 100)

	view.Append(hslScaleBox("HUE", w.colorHue))
	view.Append(hslScaleBox("SATURATION", w.colorSat))
	view.Append(hslScaleBox("LIGHTNESS", w.colorLit))

	// Preview swatch + hex label.
	w.colorSwatchProv = gtk.NewCSSProvider()
	gtk.StyleContextAddProviderForDisplay(
		gdk.DisplayGetDefault(), w.colorSwatchProv,
		gtk.STYLE_PROVIDER_PRIORITY_USER+10,
	)

	w.colorPreview = gtk.NewBox(gtk.OrientationHorizontal, 0)
	w.colorPreview.AddCSSClass("color-swatch")
	w.colorPreview.SetName("color-picker-preview")

	w.colorHexLabel = gtk.NewLabel("#FF0000")
	w.colorHexLabel.AddCSSClass("section-label")

	previewRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	previewRow.SetMarginTop(4)
	previewRow.Append(w.colorPreview)
	previewRow.Append(w.colorHexLabel)
	view.Append(previewRow)

	return view
}

// buildHSLScale creates a Scale for an HSL component.
func (w *Window) buildHSLScale(_ string, lo, hi float64) *gtk.Scale {
	sc := gtk.NewScaleWithRange(gtk.OrientationHorizontal, lo, hi, 1)
	sc.SetDigits(0)
	sc.SetDrawValue(true)
	sc.SetFocusable(false)
	sc.ConnectValueChanged(func() { w.onHSLChanged() })
	return sc
}

// hslScaleBox wraps a section label + scale into a box.
func hslScaleBox(label string, sc *gtk.Scale) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 2)
	box.Append(sectionLabel(label))
	box.Append(sc)
	return box
}

// showMainView switches the view stack to the main drawer view.
func (w *Window) showMainView() {
	if w.viewStack != nil {
		w.viewStack.SetVisibleChildName("main")
		w.swapFocusList(w.mainFocusItems)
	}
}

// showCustomView switches the view stack to the custom TDP/fan view.
// Lazy-builds the view on first access.
func (w *Window) showCustomView() {
	if w.viewStack == nil {
		return
	}
	if w.viewStack.VisibleChildName() == "custom" {
		w.showMainView()
		return
	}
	if w.customScroll == nil {
		w.viewStack.AddNamed(w.buildCustomView(), "custom")
		w.buildCustomFocusList()
	}
	w.syncCustomView()
	w.viewStack.SetVisibleChildName("custom")
	w.swapFocusList(w.customFocusItems)
	w.startTelemetryPolling()
}

// showThemeView switches the view stack to the theme picker view.
// The theme view is lazy-built on first access to keep the initial widget tree small.
func (w *Window) showThemeView() {
	if w.viewStack == nil {
		return
	}
	if w.viewStack.VisibleChildName() == "theme" {
		w.showMainView()
		return
	}
	if w.themeScroll == nil {
		w.viewStack.AddNamed(w.buildThemeView(), "theme")
		w.buildThemeFocusList()
	}
	w.viewStack.SetVisibleChildName("theme")
	w.swapFocusList(w.themeFocusItems)
}

// buildTabRow creates the Keyboard / Lightbar tab radio buttons.
func (w *Window) buildTabRow() *gtk.Box {
	row := gtk.NewBox(gtk.OrientationHorizontal, 4)

	kb := gtk.NewCheckButtonWithLabel("Keyboard")
	kb.SetActive(true)
	lb := gtk.NewCheckButtonWithLabel("Lightbar")
	lb.SetGroup(kb)

	kb.AddCSSClass("tab-btn")
	lb.AddCSSClass("tab-btn")
	kb.SetHExpand(true)
	lb.SetHExpand(true)

	kb.ConnectToggled(func() {
		if kb.Active() {
			w.tab = "keyboard"
			w.syncLightingSection()
		}
	})
	lb.ConnectToggled(func() {
		if lb.Active() {
			w.tab = "lightbar"
			w.syncLightingSection()
		}
	})

	w.tabKB = kb
	w.tabLB = lb

	if w.gamescope {
		addTouchActivate(kb, func() { kb.SetActive(true) })
		addTouchActivate(lb, func() { lb.SetActive(true) })
	}

	row.Append(kb)
	row.Append(lb)
	return row
}

// dotsPerRow is the number of accent color dots per row in the theme picker.
const dotsPerRow = 7

// modeOrder defines the display order for verified lighting mode buttons.
var modeOrder = []string{
	"static", "breathe", "cycle",
	"rainbow", "strobe",
}

// buildModeSection creates the lighting mode grid.
func (w *Window) buildModeSection() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 4)
	box.Append(sectionLabel("MODE"))

	grid := gtk.NewGrid()
	grid.SetColumnSpacing(4)
	grid.SetRowSpacing(4)
	grid.AddCSSClass("mode-grid")
	grid.AddCSSClass("btn-group")
	grid.SetColumnHomogeneous(true)

	for i, m := range modeOrder {
		mode := m
		btn := gtk.NewButtonWithLabel(strings.Title(mode)) //nolint:staticcheck // strings.Title is fine for ASCII-only mode/speed/profile labels
		btn.ConnectClicked(func() {
			setActiveButton(w.modeButtons, mode)
			w.syncModeVis()
			w.sendApply()
		})
		w.modeButtons[mode] = btn
		grid.Attach(btn, i%3, i/3, 1, 1)
	}

	box.Append(grid)
	return box
}

// setActiveButton removes .active from all buttons in the map and adds it
// to the button matching the given key. Used for button groups that replaced
// radio buttons (profiles, modes, speeds).
func setActiveButton(btns map[string]*gtk.Button, active string) {
	for k, b := range btns {
		if k == active {
			b.AddCSSClass("active")
		} else {
			b.RemoveCSSClass("active")
		}
	}
}

// buildButtonGroup creates a row of regular buttons for the given options,
// stores each in dst[option], and calls onChange when a button is clicked.
func (w *Window) buildButtonGroup(
	orientation gtk.Orientation,
	options []string,
	dst map[string]*gtk.Button,
	onChange func(string),
) *gtk.Box {
	row := gtk.NewBox(orientation, 4)
	row.AddCSSClass("btn-group")
	for _, opt := range options {
		opt := opt
		btn := gtk.NewButtonWithLabel(strings.Title(opt)) //nolint:staticcheck // strings.Title is fine for ASCII-only mode/speed/profile labels
		btn.ConnectClicked(func() {
			setActiveButton(dst, opt)
			onChange(opt)
		})
		dst[opt] = btn
		row.Append(btn)
	}
	return row
}

// buildSpeedBox creates the slow/normal/fast button row.
func (w *Window) buildSpeedBox() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 4)
	box.Append(sectionLabel("SPEED"))
	box.Append(w.buildButtonGroup(gtk.OrientationHorizontal, speeds, w.speedBtns, func(_ string) {
		w.sendApply()
	}))
	setActiveButton(w.speedBtns, "normal")
	return box
}

// buildBrightnessBox creates the brightness scale (0–3).
func (w *Window) buildBrightnessBox() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 4)
	box.Append(sectionLabel("BRIGHTNESS"))

	sc := gtk.NewScaleWithRange(gtk.OrientationHorizontal, 0, 3, 1)
	sc.SetDigits(0)
	sc.SetDrawValue(true)
	sc.SetValue(3)
	sc.SetFocusable(false)
	sc.ConnectValueChanged(func() {
		w.queueApply()
	})
	w.brightScale = sc
	box.Append(sc)
	return box
}

// profiles lists the available performance profiles. "custom" opens the TDP/fan view.
var profiles = []string{"quiet", "balanced", "performance", "custom"}

// speeds lists the available lighting animation speeds.
var speeds = []string{"slow", "normal", "fast"}

// buildProfileSection creates the 2x2 profile button grid.
func (w *Window) buildProfileSection() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 2)
	box.Append(sectionLabel("PROFILE"))

	grid := gtk.NewGrid()
	grid.SetColumnSpacing(4)
	grid.SetRowSpacing(2)
	grid.SetColumnHomogeneous(true)
	grid.AddCSSClass("btn-group")

	for i, p := range profiles {
		prof := p
		btn := gtk.NewButtonWithLabel(strings.Title(prof)) //nolint:staticcheck // strings.Title is fine for ASCII-only labels
		btn.ConnectClicked(func() {
			if prof == "custom" {
				w.showCustomView()
			} else {
				setActiveButton(w.profileBtns, prof)
				w.sendProfileSet(prof)
				go func() {
					ok, state, err := api.SendGetState()
					if ok && err == nil {
						glib.IdleAdd(func() {
							w.state = state
							w.syncing = true
							w.syncCPUPower()
							w.syncCustomView()
							w.syncing = false
						})
					}
				}()
			}
		})
		w.profileBtns[prof] = btn
		grid.Attach(btn, i%2, i/2, 1, 1)
	}

	box.Append(grid)
	return box
}

// buildBatterySection creates the battery charge limit scale (40–100%) with
// preset quick-pick buttons below it.
//
// Section label is provided by the caller (collapsibleSection wrapper in
// buildPowerTab) to avoid double-labeling when wrapped.
func (w *Window) buildBatterySection() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 4)
	box.Append(sectionLabel("BATTERY STRATEGY"))
	box.Append(w.buildBatteryPresets())
	return box
}

// batteryPresets maps preset label → percentage. Order defines UI order.
var batteryPresets = []struct {
	pct   int
	label string
}{
	{100, "Standard"},
	{80, "Balanced"},
	{60, "Max Life"},
}

// buildBatteryPresets creates the Standard / Balanced / Max Life preset row.
func (w *Window) buildBatteryPresets() *gtk.Box {
	row := gtk.NewBox(gtk.OrientationHorizontal, 4)
	row.AddCSSClass("btn-group")
	for _, p := range batteryPresets {
		p := p
		btn := gtk.NewButtonWithLabel(fmt.Sprintf("%s %d%%", p.label, p.pct))
		btn.ConnectClicked(func() {
			if w.syncing {
				return
			}
			setActiveIntButton(w.battPresetBtns, p.pct)
			w.sendBatteryLimitSet(p.pct)
		})
		w.battPresetBtns[p.pct] = btn
		row.Append(btn)
	}
	return row
}

// fanPresets defines the named fan curve presets.
var fanPresets = []string{"silent", "balanced", "turbo"}

// buildFanPresetSection creates the Silent / Balanced / Turbo fan curve
// preset row. Clicking a preset sends the corresponding curve to the daemon
// via the existing fancurve command (no new protocol).
func (w *Window) buildFanPresetSection() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 4)
	box.Append(sectionLabel("FAN MODE"))
	row := gtk.NewBox(gtk.OrientationHorizontal, 4)
	row.AddCSSClass("btn-group")
	for _, name := range fanPresets {
		name := name
		btn := gtk.NewButtonWithLabel(strings.Title(name)) //nolint:staticcheck // ASCII-only preset labels
		btn.ConnectClicked(func() {
			if w.syncing {
				return
			}
			setActiveButton(w.fanPresetBtns, name)
			w.sendFanPreset(name)
		})
		w.fanPresetBtns[name] = btn
		row.Append(btn)
	}
	box.Append(row)
	return box
}

// refreshRates defines the available eDP-1 refresh rates in display order.
var refreshRates = []int{60, 180}

// buildRefreshRateSection creates the 60Hz / 180Hz segmented control.
func (w *Window) buildRefreshRateSection() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 4)
	box.Append(sectionLabel("REFRESH RATE"))
	row := gtk.NewBox(gtk.OrientationHorizontal, 4)
	row.AddCSSClass("btn-group")
	for _, hz := range refreshRates {
		hz := hz
		btn := gtk.NewButtonWithLabel(fmt.Sprintf("%d Hz", hz))
		btn.ConnectClicked(func() {
			if w.syncing {
				return
			}
			setActiveIntButton(w.refreshBtns, hz)
			w.sendRefreshRateSet(hz)
		})
		w.refreshBtns[hz] = btn
		row.Append(btn)
	}
	box.Append(row)
	return box
}

// setActiveIntButton is the int-keyed variant of setActiveButton.
func setActiveIntButton(btns map[int]*gtk.Button, active int) {
	for k, b := range btns {
		if k == active {
			b.AddCSSClass("active")
		} else {
			b.RemoveCSSClass("active")
		}
	}
}

// buildBatteryHero builds the Overview battery card with capacity, status,
// battery power flow, health, and threshold.
func (w *Window) buildBatteryHero() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 4)

	card := gtk.NewBox(gtk.OrientationVertical, 5)
	card.AddCSSClass("card")
	card.AddCSSClass("battery-summary")
	card.SetMarginBottom(4)

	header := gtk.NewBox(gtk.OrientationHorizontal, 6)
	title := gtk.NewLabel("BATTERY")
	title.SetHAlign(gtk.AlignStart)
	title.SetHExpand(true)
	title.AddCSSClass("card-title")
	header.Append(title)
	w.battCapacityLabel = gtk.NewLabel("—")
	w.battCapacityLabel.AddCSSClass("battery-summary-value")
	header.Append(w.battCapacityLabel)
	w.battPill = gtk.NewLabel("—")
	w.battPill.AddCSSClass("pill")
	header.Append(w.battPill)
	card.Append(header)

	statusRow := gtk.NewBox(gtk.OrientationHorizontal, 6)
	w.battStatusLabel = gtk.NewLabel("—")
	w.battStatusLabel.AddCSSClass("card-sub")
	w.battStatusLabel.AddCSSClass("battery-status")
	w.battStatusLabel.SetHAlign(gtk.AlignStart)
	w.battStatusLabel.SetHExpand(true)
	statusRow.Append(w.battStatusLabel)
	w.battPowerLabel = gtk.NewLabel("—")
	w.battPowerLabel.AddCSSClass("card-sub")
	w.battPowerLabel.AddCSSClass("battery-flow")
	statusRow.Append(w.battPowerLabel)
	card.Append(statusRow)

	energyRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	energyRow.SetHomogeneous(true)
	metric, value := batteryMetric("ENERGY")
	w.battEnergyLabel = value
	energyRow.Append(metric)
	metric, value = batteryMetric("VOLTAGE")
	w.battVoltageLabel = value
	energyRow.Append(metric)
	card.Append(energyRow)

	capacityRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	capacityRow.SetHomogeneous(true)
	metric, value = batteryMetric("HEALTH")
	w.battHealthLabel = value
	capacityRow.Append(metric)
	metric, value = batteryMetric("DESIGN")
	w.battDesignLabel = value
	capacityRow.Append(metric)
	card.Append(capacityRow)

	box.Append(card)
	w.batteryHero = box
	return box
}

func batteryMetric(label string) (*gtk.Box, *gtk.Label) {
	box := gtk.NewBox(gtk.OrientationVertical, 1)
	caption := gtk.NewLabel(label)
	caption.SetHAlign(gtk.AlignStart)
	caption.AddCSSClass("battery-metric-label")
	value := gtk.NewLabel("—")
	value.SetHAlign(gtk.AlignStart)
	value.AddCSSClass("battery-metric-value")
	box.Append(caption)
	box.Append(value)
	return box, value
}

var cpuEPPs = []struct {
	value string
	label string
}{
	{"performance", "Performance"},
	{"balance_performance", "Balanced Perf"},
	{"balance_power", "Balanced Power"},
	{"power", "Power Saver"},
}

func (w *Window) buildCPUPowerSection() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 4)
	box.Append(sectionLabel("ADVANCED CPU"))

	minLabel := gtk.NewLabel("MINIMUM FREQUENCY (MHz)")
	minLabel.SetHAlign(gtk.AlignStart)
	minLabel.AddCSSClass("scale-name")
	box.Append(minLabel)
	minScale := gtk.NewScaleWithRange(gtk.OrientationHorizontal, 400, 3000, 25)
	minScale.SetDigits(0)
	minScale.SetDrawValue(true)
	minScale.SetValue(625)
	minScale.SetFocusable(false)
	w.cpuMinScale = minScale
	w.initCPUMinDebounce(minScale)
	box.Append(minScale)

	eppLabel := gtk.NewLabel("ENERGY PERFORMANCE PREFERENCE")
	eppLabel.SetHAlign(gtk.AlignStart)
	eppLabel.AddCSSClass("scale-name")
	box.Append(eppLabel)
	grid := gtk.NewGrid()
	grid.SetColumnSpacing(4)
	grid.SetRowSpacing(4)
	grid.SetColumnHomogeneous(true)
	grid.AddCSSClass("btn-group")
	for i, option := range cpuEPPs {
		option := option
		btn := gtk.NewButtonWithLabel(option.label)
		btn.ConnectClicked(func() {
			if w.syncing {
				return
			}
			setActiveButton(w.cpuEPPBtns, option.value)
			w.sendCPUEPPSet(option.value)
		})
		w.cpuEPPBtns[option.value] = btn
		grid.Attach(btn, i%2, i/2, 1, 1)
	}
	box.Append(grid)

	box.Append(w.buildToggle("CPU Boost", "Allow CPU frequencies above the nominal maximum", &w.cpuBoostSwitch, w.sendCPUBoostSet))
	box.SetVisible(false)
	w.cpuPowerBox = box
	return box
}

// colorSubBox wraps a section label + content widget into a single Box,
// making it easy to show/hide the whole subsection at once.
func colorSubBox(label string, content gtk.Widgetter) *gtk.Box {
	b := gtk.NewBox(gtk.OrientationVertical, 4)
	b.Append(sectionLabel(label))
	b.Append(content)
	return b
}

// sectionLabel creates a small-caps section label (e.g. "MODE", "SPEED").
func sectionLabel(text string) *gtk.Label {
	l := gtk.NewLabel(text)
	l.SetHAlign(gtk.AlignStart)
	l.AddCSSClass("section-label")
	return l
}

// groupLabel creates a section group heading (e.g. "TDP AND POWER", "RGB").
func groupLabel(text string) *gtk.Label {
	l := gtk.NewLabel(text)
	l.SetHAlign(gtk.AlignStart)
	l.AddCSSClass("section-group")
	return l
}

// separator creates a horizontal separator line.
func separator() *gtk.Separator {
	return gtk.NewSeparator(gtk.OrientationHorizontal)
}

// collapsibleHeader is the clickable header row of a collapsible section.
// Layout: [chevron] [LABEL] ............ [suffix]
// The chevron rotates between expanded (pan-down) and collapsed (pan-end).
// The suffix is optional and can be updated at runtime — used to show the
// current value (e.g., "TURBO" for NPU power mode) so the user can see
// the state without expanding.
type collapsibleHeader struct {
	button  *gtk.ToggleButton
	chevron *gtk.Image
	suffix  *gtk.Label
}

// newCollapsibleHeader builds the header widget. The ToggleButton wraps a
// custom child box so we control the icon + label + suffix layout precisely.
func newCollapsibleHeader(label string, defaultOpen bool) *collapsibleHeader {
	h := &collapsibleHeader{}

	contents := gtk.NewBox(gtk.OrientationHorizontal, 8)

	h.chevron = gtk.NewImageFromIconName("pan-end-symbolic")
	h.chevron.AddCSSClass("disclosure-icon")
	contents.Append(h.chevron)

	lbl := gtk.NewLabel(label)
	lbl.AddCSSClass("section-collapse-label")
	lbl.SetHAlign(gtk.AlignStart)
	contents.Append(lbl)

	spacer := gtk.NewBox(gtk.OrientationHorizontal, 0)
	spacer.SetHExpand(true)
	contents.Append(spacer)

	h.suffix = gtk.NewLabel("")
	h.suffix.AddCSSClass("collapse-suffix")
	h.suffix.SetHAlign(gtk.AlignEnd)
	h.suffix.SetVisible(false)
	contents.Append(h.suffix)

	h.button = gtk.NewToggleButton()
	h.button.SetChild(contents)
	h.button.AddCSSClass("section-collapse")
	h.button.SetActive(defaultOpen)
	h.button.SetHExpand(true)

	h.updateChevron()
	h.button.ConnectToggled(h.updateChevron)

	return h
}

// updateChevron swaps the icon based on the active (expanded) state.
func (h *collapsibleHeader) updateChevron() {
	if h.button.Active() {
		h.chevron.SetFromIconName("pan-down-symbolic")
	} else {
		h.chevron.SetFromIconName("pan-end-symbolic")
	}
}

// SetSuffix shows or hides the trailing value label. Empty string hides it.
func (h *collapsibleHeader) SetSuffix(s string) {
	if s == "" {
		h.suffix.SetVisible(false)
	} else {
		h.suffix.SetText(s)
		h.suffix.SetVisible(true)
	}
}

// collapsibleSection wraps a section label + content in a collapsible group.
// Defaults to expanded. Header meets the 40px touch target floor.
func collapsibleSection(label string, defaultOpen bool, content gtk.Widgetter) *gtk.Box {
	box, _ := collapsibleSectionWithSuffix(label, defaultOpen, content)
	return box
}

// collapsibleSectionWithSuffix is like collapsibleSection but also returns
// the header so callers can update the trailing suffix at runtime (e.g.,
// to show "TURBO" next to "NPU POWER" when collapsed).
func collapsibleSectionWithSuffix(label string, defaultOpen bool, content gtk.Widgetter) (*gtk.Box, *collapsibleHeader) {
	box := gtk.NewBox(gtk.OrientationVertical, 4)
	h := newCollapsibleHeader(label, defaultOpen)
	box.Append(h.button)

	contentWidget := gtk.BaseWidget(content)
	contentWidget.SetVisible(defaultOpen)
	h.button.ConnectToggled(func() {
		contentWidget.SetVisible(h.button.Active())
	})

	box.Append(content)
	return box, h
}

// buildMainFocusList builds the 2D focus grid for the main drawer view.
// Items are arranged by visual row/col matching the drawer layout.
func (w *Window) buildMainFocusList() {
	var items []focusItem

	// Top-level tab row — always at row 0.
	for col, name := range []string{"overview", "power", "rgb", "system"} {
		btn := w.mainTabBtns[name]
		items = append(items, focusItem{
			widget: btn, row: 0, col: col,
			section:    "main-tabs",
			onActivate: func() { btn.Activate() },
		})
	}

	// Tab-specific content.
	switch w.activeTab {
	case "overview":
		// Overview is display-only — no focusable controls below the tab row.
	case "power":
		items = append(items, w.powerTabFocusItems()...)
	case "rgb":
		items = append(items, w.rgbTabFocusItems()...)
	case "system":
		items = append(items, w.systemTabFocusItems()...)
	}

	w.mainFocusItems = items
}

// powerTabFocusItems builds the gamepad focus list for the Power tab.
func (w *Window) powerTabFocusItems() []focusItem {
	var items []focusItem
	row := 1

	// Profiles — 2x2 grid.
	for i, p := range profiles {
		btn := w.profileBtns[p]
		items = append(items, focusItem{
			widget: btn, row: row + i/2, col: i % 2,
			section:    "profile",
			onActivate: func() { btn.Activate() },
		})
	}
	row += 2

	// Fan preset buttons.
	for col, name := range fanPresets {
		btn := w.fanPresetBtns[name]
		items = append(items, focusItem{
			widget: btn, row: row, col: col,
			section:    "fan-preset",
			onActivate: func() { btn.Activate() },
		})
	}
	row++

	// Battery strategy buttons.
	for col, p := range batteryPresets {
		btn := w.battPresetBtns[p.pct]
		items = append(items, focusItem{
			widget: btn, row: row, col: col,
			section:    "battery-preset",
			onActivate: func() { btn.Activate() },
		})
	}
	row++

	if w.presetsBtn != nil {
		btn := w.presetsBtn
		items = append(items, focusItem{
			widget: btn, row: row, col: 0,
			section:    "presets",
			onActivate: func() { btn.Activate() },
		})
		row++
	}

	// Advanced CPU controls are available only for the Custom profile.
	if w.cpuPowerBox != nil {
		cpuVisible := func() bool { return w.cpuPowerBox.IsVisible() }
		cpuLeft, cpuRight, cpuGet, cpuSet := scaleAdjust(w.cpuMinScale, 25)
		items = append(items, focusItem{
			widget: w.cpuMinScale, row: row, col: 0,
			section: "cpu-power", isVisible: cpuVisible,
			editable: true,
			onLeft:   cpuLeft, onRight: cpuRight,
			getValue: cpuGet, setValue: cpuSet,
		})
		row++
		for i, option := range cpuEPPs {
			btn := w.cpuEPPBtns[option.value]
			items = append(items, focusItem{
				widget: btn, row: row + i/2, col: i % 2,
				section: "cpu-power", isVisible: cpuVisible,
				onActivate: func() { btn.Activate() },
			})
		}
		row += 2
		items = append(items, focusItem{
			widget: w.cpuBoostSwitch, row: row, col: 0,
			section: "cpu-power", isVisible: cpuVisible,
			onActivate: func() { w.cpuBoostSwitch.SetActive(!w.cpuBoostSwitch.Active()) },
		})
	}
	return items
}

// rgbTabFocusItems builds the gamepad focus list for the RGB tab.
func (w *Window) rgbTabFocusItems() []focusItem {
	var items []focusItem
	row := 1

	// Device tabs — horizontal row.
	for col, btn := range []*gtk.CheckButton{w.tabKB, w.tabLB} {
		btn := btn
		items = append(items, focusItem{
			widget: btn, row: row, col: col,
			section:    "tabs",
			onActivate: func() { btn.SetActive(true) },
		})
	}
	row++
	if w.lightingSwitch != nil {
		sw := w.lightingSwitch
		items = append(items, focusItem{
			widget: sw, row: row, col: 0,
			section:    "lighting",
			onActivate: func() { sw.SetActive(!sw.Active()) },
		})
		row++
	}
	controlsEnabled := func() bool { return w.rgbControlsBox != nil && w.rgbControlsBox.Sensitive() }

	// Mode buttons — three-column grid.
	for i, m := range modeOrder {
		btn := w.modeButtons[m]
		items = append(items, focusItem{
			widget: btn, row: row + i/3, col: i % 3,
			section: "mode", isVisible: controlsEnabled,
			onActivate: func() { btn.Activate() },
		})
	}
	row += 2

	// Color 1 presets — horizontal row of 8 buttons.
	if w.color1 != nil {
		vis := func() bool { return controlsEnabled() && w.color1Box.IsVisible() }
		for col, btn := range w.color1.presetBtns {
			btn := btn
			items = append(items, focusItem{
				widget: btn, row: row, col: col,
				section: "color1", isVisible: vis,
				onActivate: func() { btn.Activate() },
			})
		}
		row++
		items = append(items, focusItem{
			widget: w.color1.customBtn, row: row, col: 0,
			section: "color1", isVisible: vis,
			onActivate: func() { w.showColorView(w.color1) },
		})
		row++
	}

	// Color 2 presets.
	if w.color2 != nil {
		vis := func() bool { return controlsEnabled() && w.color2Box.IsVisible() }
		for col, btn := range w.color2.presetBtns {
			btn := btn
			items = append(items, focusItem{
				widget: btn, row: row, col: col,
				section: "color2", isVisible: vis,
				onActivate: func() { btn.Activate() },
			})
		}
		row++
		items = append(items, focusItem{
			widget: w.color2.customBtn, row: row, col: 0,
			section: "color2", isVisible: vis,
			onActivate: func() { w.showColorView(w.color2) },
		})
		row++
	}

	// Speed buttons — horizontal row.
	for col, s := range speeds {
		btn := w.speedBtns[s]
		items = append(items, focusItem{
			widget: btn, row: row, col: col,
			section: "speed", isVisible: func() bool { return controlsEnabled() && w.speedBox.IsVisible() },
			onActivate: func() { btn.Activate() },
		})
	}
	row++

	// Brightness slider.
	brLeft, brRight, brGet, brSet := scaleAdjust(w.brightScale, 1)
	items = append(items, focusItem{
		widget: w.brightScale, row: row, col: 0,
		section: "brightness", isVisible: func() bool { return controlsEnabled() && w.brightBox.IsVisible() },
		editable: true,
		onLeft:   brLeft, onRight: brRight,
		getValue: brGet, setValue: brSet,
	})
	return items
}

// systemTabFocusItems builds the gamepad focus list for the System tab.
func (w *Window) systemTabFocusItems() []focusItem {
	var items []focusItem
	row := 1

	// Refresh rate buttons.
	for col, hz := range refreshRates {
		btn := w.refreshBtns[hz]
		items = append(items, focusItem{
			widget: btn, row: row, col: col,
			section:    "refresh-rate",
			onActivate: func() { btn.Activate() },
		})
	}
	row++

	if w.overdriveSwitch != nil {
		sw := w.overdriveSwitch
		items = append(items, focusItem{
			widget: sw, row: row, col: 0,
			section:    "system",
			onActivate: func() { sw.SetActive(!sw.Active()) },
		})
		row++
	}
	if w.bootSoundSwitch != nil {
		sw := w.bootSoundSwitch
		items = append(items, focusItem{
			widget: sw, row: row, col: 0,
			section:    "system",
			onActivate: func() { sw.SetActive(!sw.Active()) },
		})
		row++
	}
	if w.paletteBtn != nil {
		btn := w.paletteBtn
		items = append(items, focusItem{
			widget: btn, row: row, col: 0,
			section:    "appearance",
			onActivate: func() { btn.Activate() },
		})
	}
	return items
}

// buildThemeFocusList builds the 2D focus grid for the theme picker view.
// Must be called after appendThemeChoices has populated w.themeRadios/w.themeDots.
func (w *Window) buildThemeFocusList() {
	var items []focusItem

	// Row 0: back button.
	if w.themeBackBtn != nil {
		items = append(items, focusItem{
			widget: w.themeBackBtn, row: 0, col: 0,
			section:    "nav",
			onActivate: func() { w.showMainView() },
		})
	}

	row := 1
	for i, btn := range w.themeRadios {
		btn := btn
		items = append(items, focusItem{
			widget: btn, row: row, col: 0,
			section:    "theme",
			onActivate: func() { btn.SetActive(true) },
		})
		row++

		// Accent dots for this theme.
		if i < len(w.themeDots) && len(w.themeDots[i]) > 0 {
			for j, dot := range w.themeDots[i] {
				dot := dot
				items = append(items, focusItem{
					widget: dot, row: row + j/dotsPerRow, col: j % dotsPerRow,
					section:    "theme",
					onActivate: func() { dot.Activate() },
				})
			}
			row += (len(w.themeDots[i])-1)/dotsPerRow + 1
		}
	}

	w.themeFocusItems = items
}

// buildColorFocusList builds the 2D focus grid for the HSL color picker view.
func (w *Window) buildColorFocusList() {
	var items []focusItem

	// Row 0: back button.
	if w.colorBackBtn != nil {
		items = append(items, focusItem{
			widget: w.colorBackBtn, row: 0, col: 0,
			section:    "nav",
			onActivate: func() { w.showMainView() },
		})
	}

	// Row 1: color presets.
	for col, btn := range w.colorPickerPresets {
		btn := btn
		items = append(items, focusItem{
			widget: btn, row: 1, col: col,
			section:    "presets",
			onActivate: func() { btn.Activate() },
		})
	}

	// Rows 2-4: HSL sliders (editable).
	for i, sc := range []*gtk.Scale{w.colorHue, w.colorSat, w.colorLit} {
		oL, oR, gV, sV := scaleAdjust(sc, 5)
		items = append(items, focusItem{
			widget: sc, row: 2 + i, col: 0,
			section:  "sliders",
			editable: true,
			onLeft:   oL, onRight: oR,
			getValue: gV, setValue: sV,
		})
	}

	w.colorFocusItems = items
}
