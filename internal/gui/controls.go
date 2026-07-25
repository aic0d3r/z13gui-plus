package gui

// controls.go — builds the entire drawer widget tree, theme picker view,
// and HSL color picker view. All views live in a gtk.Stack (both KDE and
// gamescope modes) for consistent gamepad navigation.

import (
	"fmt"
	"strings"
	"time"

	"github.com/dahui/z13ctl/api"
	"github.com/dahui/z13gui/internal/theme"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
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

// buildPowerTab builds the Power tab. Telemetry lives on Overview; this tab
// groups current policy status with the controls that change it.
func (w *Window) buildPowerTab() *gtk.ScrolledWindow {
	inner := gtk.NewBox(gtk.OrientationVertical, 6)
	inner.SetMarginTop(2)
	inner.SetMarginBottom(4)
	inner.SetMarginStart(12)
	inner.SetMarginEnd(12)

	inner.Append(w.buildProfileSection())
	inner.Append(w.buildFanPresetSection())
	inner.Append(w.buildPresetEntrySection())
	inner.Append(w.buildBatterySection())
	inner.Append(w.buildAdvancedTuningSection())

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(inner)
	w.powerScroll = scroll
	return scroll
}

// buildRGBTab builds the RGB tab: device tabs, mode, colors, speed, brightness.
func (w *Window) buildRGBTab() *gtk.ScrolledWindow {
	inner := gtk.NewBox(gtk.OrientationVertical, 6)
	inner.SetMarginTop(2)
	inner.SetMarginBottom(4)
	inner.SetMarginStart(12)
	inner.SetMarginEnd(12)

	lighting, summary := statusCard("LIGHTING")
	w.lightingSummary = summary
	lighting.Append(sectionLabel("DEVICE"))
	lighting.Append(w.buildTabRow())
	lighting.Append(w.buildToggle("Lighting", "Turn lighting on or off for the selected device", &w.lightingSwitch, w.setLightingEnabled))
	inner.Append(lighting)

	effect, summary := statusCard("EFFECT")
	w.rgbEffectSummary = summary
	w.rgbEffectCard = effect
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
	effect.Append(controls)
	inner.Append(effect)

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
	inner := gtk.NewBox(gtk.OrientationVertical, 6)
	inner.SetMarginTop(2)
	inner.SetMarginBottom(4)
	inner.SetMarginStart(12)
	inner.SetMarginEnd(12)

	display, summary := statusCard("DISPLAY")
	w.displaySummary = summary
	display.Append(w.buildRefreshRateSection())
	display.Append(w.buildToggle("Panel Overdrive", "Enable panel overdrive for faster pixel response (may increase power use)", &w.overdriveSwitch, func(active bool) {
		v := 0
		if active {
			v = 1
		}
		w.runStateActionQuiet("panel overdrive", func() (bool, error) {
			return api.SendPanelOverdriveSet(v)
		}, func() {
			if w.state != nil {
				w.state.PanelOverdrive = v
			}
		}, nil)
	}))
	inner.Append(display)

	startup, summary := statusCard("STARTUP")
	w.startupSummary = summary
	startup.Append(w.buildToggle("Boot Sound", "Play startup sound when the laptop powers on", &w.bootSoundSwitch, func(active bool) {
		v := 0
		if active {
			v = 1
		}
		w.runStateActionQuiet("boot sound", func() (bool, error) {
			return api.SendBootSoundSet(v)
		}, func() {
			if w.state != nil {
				w.state.BootSound = v
			}
			setOnOffSummary(w.startupSummary, active)
		}, nil)
	}))
	inner.Append(startup)
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
	box, summary := statusCard("APPEARANCE")
	box.AddCSSClass("btn-group")

	cfg := theme.LoadAppConfig()
	w.appearanceSummary = summary
	w.appearanceSummary.SetLabel(themeButtonLabel(cfg.Theme, cfg.Accent, w.isCustomTheme))
	w.paletteBtn = gtk.NewButtonWithLabel("Choose Theme")
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
	s.Connect("notify::active", func() {
		if s.Active() {
			box.AddCSSClass("toggle-active")
		} else {
			box.RemoveCSSClass("toggle-active")
		}
	})
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

	content := gtk.NewBox(gtk.OrientationVertical, 6)
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
		dot.ConnectClicked(func() {
			onClick(ac)
			for _, d := range dots {
				d.RemoveCSSClass("accent-dot-active")
			}
			dot.AddCSSClass("accent-dot-active")
		})
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

	content := gtk.NewBox(gtk.OrientationVertical, 8)

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
	content.Append(presetsRow)

	// HSL sliders.
	w.colorHue = w.buildHSLScale("HUE", 0, 360)
	w.colorSat = w.buildHSLScale("SATURATION", 0, 100)
	w.colorLit = w.buildHSLScale("LIGHTNESS", 0, 100)

	content.Append(hslScaleBox("HUE", w.colorHue))
	content.Append(hslScaleBox("SATURATION", w.colorSat))
	content.Append(hslScaleBox("LIGHTNESS", w.colorLit))

	// Preview swatch + hex label.
	w.colorSwatchProv = gtk.NewCSSProvider()
	gtk.StyleContextAddProviderForDisplay(
		gdk.DisplayGetDefault(), w.colorSwatchProv,
		gtk.STYLE_PROVIDER_PRIORITY_USER+10,
	)

	w.colorPreview = gtk.NewBox(gtk.OrientationHorizontal, 0)
	w.colorPreview.AddCSSClass("color-preview")
	w.colorPreview.SetName("color-picker-preview")

	w.colorHexLabel = gtk.NewLabel("#FF0000")
	w.colorHexLabel.AddCSSClass("scale-value")

	previewRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	previewRow.SetMarginTop(4)
	previewRow.Append(w.colorPreview)
	w.colorHexLabel.SetHExpand(true)
	previewRow.Append(w.colorHexLabel)
	content.Append(previewRow)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(content)
	w.colorScroll = scroll
	view.Append(scroll)

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

func (w *Window) backCurrentView() {
	if w.viewStack == nil {
		return
	}
	switch w.viewStack.VisibleChildName() {
	case "chooser":
		w.showPresetsView()
	case "confirm":
		w.returnFromConfirmation()
	default:
		w.showMainView()
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
	row.SetHomogeneous(true)
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

// profiles lists the firmware performance profiles.
var profiles = []string{"quiet", "balanced", "performance"}

// speeds lists the available lighting animation speeds.
var speeds = []string{"slow", "normal", "fast"}

func statusCard(title string) (*gtk.Box, *gtk.Label) {
	card := gtk.NewBox(gtk.OrientationVertical, 6)
	card.AddCSSClass("card")
	header := gtk.NewBox(gtk.OrientationHorizontal, 6)
	label := gtk.NewLabel(title)
	label.SetHAlign(gtk.AlignStart)
	label.SetHExpand(true)
	label.AddCSSClass("card-title")
	header.Append(label)
	summary := gtk.NewLabel("—")
	summary.AddCSSClass("pill")
	header.Append(summary)
	card.Append(header)
	return card, summary
}

// buildProfileSection creates the stock profile button row.
func (w *Window) buildProfileSection() *gtk.Box {
	box, summary := statusCard("PROFILE")
	w.profileSummary = summary

	grid := gtk.NewGrid()
	grid.SetColumnSpacing(4)
	grid.SetColumnHomogeneous(true)
	grid.AddCSSClass("btn-group")

	for i, p := range profiles {
		prof := p
		btn := gtk.NewButtonWithLabel(strings.Title(prof)) //nolint:staticcheck // strings.Title is fine for ASCII-only labels
		btn.ConnectClicked(func() {
			w.runStateAction("set profile", func() (bool, error) { return api.SendProfileSet(prof) })
		})
		w.profileBtns[prof] = btn
		grid.Attach(btn, i, 0, 1, 1)
	}

	box.Append(grid)
	return box
}

// buildBatterySection creates the charge-limit strategy card.
func (w *Window) buildBatterySection() *gtk.Box {
	box, summary := statusCard("BATTERY STRATEGY")
	w.batterySummary = summary
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
	row.SetHomogeneous(true)
	for _, p := range batteryPresets {
		p := p
		btn := gtk.NewButtonWithLabel(fmt.Sprintf("%s %d%%", p.label, p.pct))
		btn.ConnectClicked(func() {
			if w.syncing {
				return
			}
			w.runStateAction("set battery strategy", func() (bool, error) {
				return api.SendBatteryLimitSet(p.pct)
			})
		})
		w.battPresetBtns[p.pct] = btn
		row.Append(btn)
	}
	return row
}

// fanPresets defines firmware Auto followed by the named custom curves.
var fanPresets = []string{"auto", "silent", "balanced", "turbo"}

// buildFanPresetSection creates firmware Auto and custom fan-mode controls.
func (w *Window) buildFanPresetSection() *gtk.Box {
	box, summary := statusCard("FAN MODE")
	w.fanSummary = summary
	row := gtk.NewBox(gtk.OrientationHorizontal, 4)
	row.AddCSSClass("btn-group")
	row.SetHomogeneous(true)
	for _, name := range fanPresets {
		name := name
		btn := gtk.NewButtonWithLabel(strings.Title(name)) //nolint:staticcheck // ASCII-only preset labels
		btn.ConnectClicked(func() {
			if w.syncing {
				return
			}
			if name == "auto" {
				w.runStateAction("restore automatic fan control", api.SendFanCurveReset)
				return
			}
			points := fanPresetPoints(name)
			w.runStateAction("set fan mode", func() (bool, error) { return api.SendFanCurveSet(points) })
		})
		w.fanPresetBtns[name] = btn
		row.Append(btn)
	}
	box.Append(row)
	w.fanSafetyLabel = gtk.NewLabel("")
	w.fanSafetyLabel.SetHAlign(gtk.AlignStart)
	w.fanSafetyLabel.SetWrap(true)
	w.fanSafetyLabel.AddCSSClass("card-sub")
	w.fanSafetyLabel.SetVisible(false)
	box.Append(w.fanSafetyLabel)
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
	row.SetHomogeneous(true)
	for _, hz := range refreshRates {
		hz := hz
		btn := gtk.NewButtonWithLabel(fmt.Sprintf("%d Hz", hz))
		btn.ConnectClicked(func() {
			if w.syncing {
				return
			}
			w.pendingRefreshRate = hz
			w.refreshPendingUntil = time.Now().Add(5 * time.Second)
			setActiveIntButton(w.refreshBtns, hz)
			w.runStateActionQuiet("refresh rate", func() (bool, error) {
				return api.SendRefreshRateSet(hz)
			}, func() {
				if w.pendingRefreshRate != 0 {
					setActiveIntButton(w.refreshBtns, w.pendingRefreshRate)
					w.displaySummary.SetLabel(fmt.Sprintf("%d HZ", w.pendingRefreshRate))
				}
			}, func() {
				if w.pendingRefreshRate == hz {
					w.pendingRefreshRate = 0
				}
			})
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

	header := gtk.NewBox(gtk.OrientationHorizontal, 6)
	title := gtk.NewLabel("BATTERY")
	title.SetHAlign(gtk.AlignStart)
	title.SetHExpand(true)
	title.AddCSSClass("card-title")
	header.Append(title)
	w.battCapacityLabel = gtk.NewLabel("—")
	w.battCapacityLabel.AddCSSClass("battery-summary-value")
	header.Append(w.battCapacityLabel)
	card.Append(header)

	statusRow := gtk.NewBox(gtk.OrientationHorizontal, 6)
	w.battStatusLabel = gtk.NewLabel("—")
	w.battStatusLabel.AddCSSClass("card-sub")
	w.battStatusLabel.AddCSSClass("battery-status")
	w.battStatusLabel.SetHAlign(gtk.AlignStart)
	w.battStatusLabel.SetHExpand(true)
	statusRow.Append(w.battStatusLabel)
	card.Append(statusRow)

	strategyRow := gtk.NewBox(gtk.OrientationHorizontal, 6)
	stratCaption := gtk.NewLabel("STRATEGY")
	stratCaption.SetHAlign(gtk.AlignStart)
	stratCaption.SetHExpand(true)
	stratCaption.AddCSSClass("battery-metric-label")
	strategyRow.Append(stratCaption)
	w.battPill = gtk.NewLabel("—")
	w.battPill.AddCSSClass("pill")
	strategyRow.Append(w.battPill)
	card.Append(strategyRow)

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

	powerRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	powerRow.SetHomogeneous(true)
	metric, value = batteryMetric("DRAW")
	w.battDrawLabel = value
	powerRow.Append(metric)
	metric, value = batteryMetric("TIME")
	w.battTimeLabel = value
	powerRow.Append(metric)
	card.Append(powerRow)

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
	{"balance_performance", "Responsive"},
	{"balance_power", "Efficient"},
	{"power", "Power Saver"},
}

func (w *Window) buildAdvancedTuningSection() *gtk.Box {
	content := gtk.NewBox(gtk.OrientationVertical, 6)
	detail := gtk.NewLabel("CPU behavior and independent TDP, fan curve, and undervolt overrides")
	detail.SetHAlign(gtk.AlignStart)
	detail.SetWrap(true)
	detail.AddCSSClass("card-sub")
	content.Append(detail)

	minLabel := gtk.NewLabel("MINIMUM CPU FREQUENCY (MHz)")
	minLabel.SetHAlign(gtk.AlignStart)
	minLabel.AddCSSClass("scale-name")
	content.Append(minLabel)
	w.cpuMinScale = gtk.NewScaleWithRange(gtk.OrientationHorizontal, 400, 3000, 25)
	w.cpuMinScale.SetDigits(0)
	w.cpuMinScale.SetDrawValue(true)
	w.cpuMinScale.SetFocusable(false)
	w.initCPUMinDebounce(w.cpuMinScale)
	content.Append(w.cpuMinScale)

	eppLabel := gtk.NewLabel("ENERGY PREFERENCE")
	eppLabel.SetHAlign(gtk.AlignStart)
	eppLabel.AddCSSClass("scale-name")
	content.Append(eppLabel)
	grid := gtk.NewGrid()
	grid.SetColumnSpacing(4)
	grid.SetRowSpacing(4)
	grid.SetColumnHomogeneous(true)
	grid.AddCSSClass("btn-group")
	for i, option := range cpuEPPs {
		option := option
		btn := gtk.NewButtonWithLabel(option.label)
		btn.ConnectClicked(func() {
			if !w.syncing {
				w.runStateAction("set CPU energy preference", func() (bool, error) { return api.SendCPUEPPSet(option.value) })
			}
		})
		w.cpuEPPBtns[option.value] = btn
		grid.Attach(btn, i%2, i/2, 1, 1)
	}
	content.Append(grid)
	content.Append(w.buildToggle("CPU Boost", "Allow frequencies above the nominal maximum", &w.cpuBoostSwitch, func(enabled bool) {
		w.runStateAction("set CPU boost", func() (bool, error) { return api.SendCPUBoostSet(enabled) })
	}))
	content.Append(separator())
	w.tuningBtn = gtk.NewButtonWithLabel("Open tuning")
	w.tuningBtn.AddCSSClass("action-btn")
	w.tuningBtn.ConnectClicked(func() { w.showCustomView() })
	content.Append(w.tuningBtn)
	box := gtk.NewBox(gtk.OrientationVertical, 4)
	box.AddCSSClass("card")
	header := newCollapsibleHeader("ADVANCED TUNING", false)
	box.Append(header.button)
	w.tuningSummary = gtk.NewLabel("TDP FIRMWARE · UV STOCK")
	w.tuningSummary.SetHAlign(gtk.AlignStart)
	w.tuningSummary.AddCSSClass("preset-summary")
	box.Append(w.tuningSummary)
	content.SetVisible(false)
	header.button.ConnectToggled(func() { content.SetVisible(header.button.Active()) })
	box.Append(content)
	w.tuningHeader = header
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

	// Profiles.
	for i, p := range profiles {
		btn := w.profileBtns[p]
		items = append(items, focusItem{
			widget: btn, row: row, col: i,
			section:    "profile",
			onActivate: func() { btn.Activate() },
		})
	}
	row++

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

	if w.presetsBtn != nil {
		btn := w.presetsBtn
		items = append(items, focusItem{
			widget: btn, row: row, col: 0,
			section:    "presets",
			onActivate: func() { btn.Activate() },
		})
		row++
	}

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

	if w.tuningHeader != nil {
		btn := w.tuningHeader.button
		items = append(items, focusItem{
			widget: btn, row: row, col: 0,
			section:    "tuning",
			onActivate: func() { btn.SetActive(!btn.Active()) },
		})
		row++
	}
	tuningVisible := func() bool { return w.tuningHeader != nil && w.tuningHeader.button.Active() }
	if w.cpuMinScale != nil {
		left, right, get, set := scaleAdjust(w.cpuMinScale, 25)
		items = append(items, focusItem{
			widget: w.cpuMinScale, row: row, col: 0,
			section: "tuning", isVisible: tuningVisible, editable: true,
			onLeft: left, onRight: right, getValue: get, setValue: set,
		})
		row++
	}
	for i, option := range cpuEPPs {
		btn := w.cpuEPPBtns[option.value]
		items = append(items, focusItem{
			widget: btn, row: row + i/2, col: i % 2,
			section: "tuning", isVisible: tuningVisible,
			onActivate: func() { btn.Activate() },
		})
	}
	row += 2
	if w.cpuBoostSwitch != nil {
		items = append(items, focusItem{
			widget: w.cpuBoostSwitch, row: row, col: 0,
			section: "tuning", isVisible: tuningVisible,
			onActivate: func() { w.cpuBoostSwitch.SetActive(!w.cpuBoostSwitch.Active()) },
		})
		row++
	}
	if w.tuningBtn != nil {
		btn := w.tuningBtn
		items = append(items, focusItem{
			widget: btn, row: row, col: 0,
			section: "tuning", isVisible: tuningVisible,
			onActivate: func() { btn.Activate() },
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
