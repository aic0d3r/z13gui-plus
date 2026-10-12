package gui

// preset_editor.go - preset detail view: edit a preset's values directly.

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/dahui/z13ctl/api"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type choice struct{ value, label string }

// presetHighTDP reports whether applying p engages high-TDP safety cooling.
func presetHighTDP(p api.Preset) bool {
	return p.TDP != nil && p.TDP.PL1SPL > tdpMaxSafe
}

func clonePresetValue(in api.Preset) api.Preset {
	out := in
	out.FanCurve = nil
	if in.TDP != nil {
		t := *in.TDP
		out.TDP = &t
	}
	if in.Undervolt != nil {
		u := *in.Undervolt
		out.Undervolt = &u
	}
	if in.CPUPower != nil {
		c := *in.CPUPower
		out.CPUPower = &c
	}
	if in.PanelOverdrive != nil {
		v := *in.PanelOverdrive
		out.PanelOverdrive = &v
	}
	return out
}

// presetEditor collects the widgets and focus order of one rebuild.
type presetEditor struct {
	box   *gtk.Box
	focus []focusItem
	row   int
}

func (e *presetEditor) heading(text string) {
	e.box.Append(sectionLabel(text))
}

// choices appends a button row (a two-column grid for more than three
// options); the selected value is highlighted and onSelect runs on click.
func (e *presetEditor) choices(parent *gtk.Box, options []choice, selected string, visible func() bool, onSelect func(string)) {
	cols := len(options)
	if cols > 3 {
		cols = 2
	}
	grid := gtk.NewGrid()
	grid.SetColumnSpacing(4)
	grid.SetRowSpacing(4)
	grid.SetColumnHomogeneous(true)
	grid.AddCSSClass("btn-group")
	btns := make(map[string]*gtk.Button, len(options))
	for i, opt := range options {
		btn := gtk.NewButtonWithLabel(opt.label)
		btn.SetHExpand(true)
		btn.ConnectClicked(func() {
			setActiveButton(btns, opt.value)
			onSelect(opt.value)
		})
		btns[opt.value] = btn
		grid.Attach(btn, i%cols, i/cols, 1, 1)
		e.focus = append(e.focus, focusItem{
			widget: btn, row: e.row + i/cols, col: i % cols, section: "preset-editor", isVisible: visible,
			onActivate: func() { btn.Activate() },
		})
	}
	setActiveButton(btns, selected)
	parent.Append(grid)
	e.row += (len(options) + cols - 1) / cols
}

// scale appends a labelled slider to parent.
func (e *presetEditor) scale(parent *gtk.Box, lo, hi, step float64, value int, format func(int) string, visible func() bool, onChange func(int)) {
	label := gtk.NewLabel(format(value))
	label.SetHAlign(gtk.AlignStart)
	label.AddCSSClass("scale-value")
	sc := gtk.NewScaleWithRange(gtk.OrientationHorizontal, lo, hi, step)
	sc.SetDigits(0)
	sc.SetDrawValue(false)
	sc.SetFocusable(false)
	sc.SetValue(float64(value))
	sc.ConnectValueChanged(func() {
		v := int(sc.Value())
		label.SetLabel(format(v))
		onChange(v)
	})
	parent.Append(label)
	parent.Append(sc)
	left, right, get, set := scaleAdjust(sc, step)
	e.focus = append(e.focus, focusItem{
		widget: sc, row: e.row, col: 0, section: "preset-editor", isVisible: visible, editable: true,
		onLeft: left, onRight: right, getValue: get, setValue: set,
	})
	e.row++
}

// sub appends a section that can be hidden; visible reports its state for
// gamepad focus.
func (e *presetEditor) sub() (box *gtk.Box, visible func() bool) {
	box = gtk.NewBox(gtk.OrientationVertical, 4)
	e.box.Append(box)
	return box, box.Visible
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
	name := w.presetDetailName
	w.presetDetailTitle.SetLabel(name)
	w.presetDraftBase = clonePresetValue(preset)
	draft := clonePresetValue(preset)
	draft.BaseProfile = ""
	w.presetDraftBase.BaseProfile = ""

	e := &presetEditor{box: w.presetDetailContent, row: 1}
	w.presetDetailUsage = presetUsage(name, w.state)
	usage := gtk.NewLabel(w.presetDetailUsage)
	usage.SetHAlign(gtk.AlignStart)
	usage.AddCSSClass("card-sub")
	e.box.Append(usage)

	save := gtk.NewButtonWithLabel("Save")
	if w.state.ActivePreset == name {
		save.SetLabel("Save & Apply")
	}
	e.heading("NAME")
	nameEntry := gtk.NewEntry()
	nameEntry.SetText(name)
	e.box.Append(nameEntry)
	e.focus = append(e.focus, focusItem{widget: nameEntry, row: e.row, col: 0, section: "preset-editor", onActivate: func() { nameEntry.GrabFocus() }})
	e.row++
	safety := gtk.NewLabel("Sustained TDP above 75 W: fans lock to high-TDP safety cooling while this preset is active.")
	safety.SetHAlign(gtk.AlignStart)
	safety.SetWrap(true)
	safety.AddCSSClass("card-sub")
	safety.AddCSSClass("warning")
	newName := func() string { return strings.TrimSpace(nameEntry.Text()) }
	w.presetEditorDirty = func() bool { return newName() != name || !reflect.DeepEqual(draft, w.presetDraftBase) }
	changed := func() {
		save.SetSensitive(newName() != name || !reflect.DeepEqual(draft, w.presetDraftBase))
		safety.SetVisible(presetHighTDP(draft))
	}

	e.heading("PROFILE")
	e.choices(e.box, []choice{{"quiet", "Quiet"}, {"balanced", "Balanced"}, {"performance", "Performance"}}, draft.Profile, nil, func(v string) {
		draft.Profile = v
		changed()
	})

	// TDP: firmware default for the profile, or explicit PL1/PL2/PL3.
	e.heading("TDP")
	tdpMode := "default"
	if draft.TDP != nil {
		tdpMode = "custom"
	}
	tdpValues := api.TDPState{PL1SPL: 50, PL2SPPT: 50, FPPT: 50}
	if draft.TDP != nil {
		tdpValues = *draft.TDP
	} else if w.state.TDP != nil && w.state.TDP.PL1SPL >= tdpMin {
		tdpValues = *w.state.TDP
	}
	var tdpBox *gtk.Box
	var tdpVisible func() bool
	e.choices(e.box, []choice{{"default", "Profile Default"}, {"custom", "Custom"}}, tdpMode, nil, func(v string) {
		if v == "custom" {
			t := tdpValues
			draft.TDP = &t
		} else {
			draft.TDP = nil
		}
		tdpBox.SetVisible(v == "custom")
		changed()
	})
	tdpBox, tdpVisible = e.sub()
	watts := func(prefix string) func(int) string {
		return func(v int) string { return fmt.Sprintf("%s: %d W", prefix, v) }
	}
	setTDP := func(apply func(*api.TDPState, int)) func(int) {
		return func(v int) {
			apply(&tdpValues, v)
			if draft.TDP != nil {
				apply(draft.TDP, v)
			}
			changed()
		}
	}
	e.scale(tdpBox, tdpMin, tdpMaxAdvanced, 1, tdpValues.PL1SPL, watts("PL1 sustained"), tdpVisible, setTDP(func(t *api.TDPState, v int) { t.PL1SPL = v }))
	e.scale(tdpBox, tdpMin, tdpMaxAdvanced, 1, tdpValues.PL2SPPT, watts("PL2 boost"), tdpVisible, setTDP(func(t *api.TDPState, v int) { t.PL2SPPT = v }))
	e.scale(tdpBox, tdpMin, tdpMaxAdvanced, 1, tdpValues.FPPT, watts("PL3 peak"), tdpVisible, setTDP(func(t *api.TDPState, v int) { t.FPPT = v }))
	tdpBox.SetVisible(tdpMode == "custom")
	e.box.Append(safety)

	// Undervolt: only offered where ryzen_smu works (the daemon rejects it
	// otherwise), or when the preset already carries one.
	if w.state.UndervoltAvailable || draft.Undervolt != nil {
		e.heading("UNDERVOLT")
		uvMode := "stock"
		if draft.Undervolt != nil {
			uvMode = "custom"
		}
		uvValue := -10
		if draft.Undervolt != nil {
			uvValue = draft.Undervolt.CPUCO
		} else if w.state.Undervolt != nil && w.state.Undervolt.CPUCO != 0 {
			uvValue = w.state.Undervolt.CPUCO
		}
		var uvBox *gtk.Box
		e.choices(e.box, []choice{{"stock", "Stock"}, {"custom", "Custom"}}, uvMode, nil, func(v string) {
			if v == "custom" {
				draft.Undervolt = &api.UndervoltState{CPUCO: uvValue}
			} else {
				draft.Undervolt = nil
			}
			uvBox.SetVisible(v == "custom")
			changed()
		})
		box, visible := e.sub()
		uvBox = box
		e.scale(uvBox, -40, 0, 1, uvValue, func(v int) string { return uvLabel("CPU Curve Optimizer", v) }, visible, func(v int) {
			uvValue = v
			if draft.Undervolt != nil {
				draft.Undervolt.CPUCO = v
			}
			changed()
		})
		uvBox.SetVisible(uvMode == "custom")
	}

	// CPU behavior: keep whatever is current, or set min frequency/EPP/boost.
	if cpuState := w.state.CPUPower; cpuState != nil || draft.CPUPower != nil {
		e.heading("CPU BEHAVIOR")
		cpuMode := "keep"
		if draft.CPUPower != nil {
			cpuMode = "set"
		}
		cpuValues := api.CPUPowerPreset{MinFrequencyKHz: 625000, EPP: "balance_performance", Boost: true}
		if draft.CPUPower != nil {
			cpuValues = *draft.CPUPower
		} else if cpuState != nil {
			cpuValues = api.CPUPowerPreset{MinFrequencyKHz: cpuState.MinFrequencyKHz, EPP: kernelEPP(cpuState.EPP), Boost: cpuState.Boost}
		}
		var cpuBox *gtk.Box
		e.choices(e.box, []choice{{"keep", "Keep Current"}, {"set", "Set"}}, cpuMode, nil, func(v string) {
			if v == "set" {
				c := cpuValues
				draft.CPUPower = &c
			} else {
				draft.CPUPower = nil
			}
			cpuBox.SetVisible(v == "set")
			changed()
		})
		box, visible := e.sub()
		cpuBox = box
		setCPU := func(apply func(*api.CPUPowerPreset)) {
			apply(&cpuValues)
			if draft.CPUPower != nil {
				apply(draft.CPUPower)
			}
			changed()
		}
		loMHz, hiMHz := 400.0, 5000.0
		var eppChoices []string
		if cpuState != nil && cpuState.MaxLimitKHz > 0 {
			loMHz, hiMHz = float64(cpuState.MinLimitKHz)/1000, float64(cpuState.MaxLimitKHz)/1000
			eppChoices = cpuState.EPPChoices
		}
		e.scale(cpuBox, loMHz, hiMHz, 25, cpuValues.MinFrequencyKHz/1000, func(v int) string { return fmt.Sprintf("Minimum frequency: %d MHz", v) }, visible, func(v int) {
			setCPU(func(c *api.CPUPowerPreset) { c.MinFrequencyKHz = v * 1000 })
		})
		var epps []choice
		for _, option := range cpuEPPs {
			if eppSupported(eppChoices, option.value) || option.value == cpuValues.EPP {
				epps = append(epps, choice{option.value, option.label})
			}
		}
		e.choices(cpuBox, epps, cpuValues.EPP, visible, func(v string) {
			setCPU(func(c *api.CPUPowerPreset) { c.EPP = v })
		})
		boost := "off"
		if cpuValues.Boost {
			boost = "on"
		}
		e.choices(cpuBox, []choice{{"off", "Boost Off"}, {"on", "Boost On"}}, boost, visible, func(v string) {
			setCPU(func(c *api.CPUPowerPreset) { c.Boost = v == "on" })
		})
		cpuBox.SetVisible(cpuMode == "set")
	}

	e.heading("REFRESH RATE")
	refresh := fmt.Sprint(draft.RefreshRate)
	e.choices(e.box, []choice{{"0", "Keep"}, {"60", "60 Hz"}, {"180", "180 Hz"}}, refresh, nil, func(v string) {
		_, _ = fmt.Sscan(v, &draft.RefreshRate)
		changed()
	})

	e.heading("PANEL OVERDRIVE")
	overdrive := "keep"
	if draft.PanelOverdrive != nil {
		overdrive = fmt.Sprint(*draft.PanelOverdrive)
	}
	e.choices(e.box, []choice{{"keep", "Keep"}, {"0", "Off"}, {"1", "On"}}, overdrive, nil, func(v string) {
		if v == "keep" {
			draft.PanelOverdrive = nil
		} else {
			n := 0
			if v == "1" {
				n = 1
			}
			draft.PanelOverdrive = &n
		}
		changed()
	})

	save.AddCSSClass("suggested-action")
	nameEntry.ConnectChanged(changed)
	save.ConnectClicked(func() {
		value := clonePresetValue(draft)
		target := newName()
		if target == "" {
			w.setPresetStatus("Enter a preset name")
			return
		}
		if target != name && hasPreset(w.state, target) {
			w.setPresetStatus(fmt.Sprintf("A preset named %q already exists", target))
			return
		}
		valuesChanged := !reflect.DeepEqual(draft, w.presetDraftBase)
		message := target + " saved"
		if w.state.ActivePreset == name && valuesChanged {
			message = target + " saved and applied"
		}
		w.queueStateAction(stateAction{
			name: "save preset", reportStatus: true,
			fn: func() (bool, error) {
				if target != name {
					if ok, err := api.SendPresetRename(name, target); !ok || err != nil {
						return ok, err
					}
				}
				if !valuesChanged {
					return true, nil
				}
				return api.SendPresetSet(target, value)
			},
			onApplied: func() {
				w.presetEditorDirty = nil
				w.showPresetsView()
				w.showPresetFeedback(message)
			},
		})
	})
	e.box.Append(save)
	e.focus = append(e.focus, focusItem{widget: save, row: e.row, col: 0, section: "preset-actions", onActivate: func() { save.Activate() }})
	e.row++

	update := gtk.NewButtonWithLabel("Update from Current Settings")
	update.AddCSSClass("action-btn")
	update.ConnectClicked(func() {
		w.showConfirmation("Update Preset", fmt.Sprintf("Replace %q with the settings active right now?", name), "Replace", "preset-detail", false, func() {
			w.runStateAction("update preset", func() (bool, error) { return api.SendPresetSave(name) })
		})
	})
	e.box.Append(update)
	e.focus = append(e.focus, focusItem{widget: update, row: e.row, col: 0, section: "preset-actions", onActivate: func() { update.Activate() }})
	e.row++

	remove := gtk.NewButtonWithLabel("Delete Preset")
	remove.AddCSSClass("destructive-action")
	assigned := presetAssigned(name, w.state)
	remove.SetSensitive(!assigned)
	remove.ConnectClicked(func() {
		w.showConfirmation("Delete Preset", fmt.Sprintf("Delete %q? Current hardware settings will not change.", name), "Delete", "presets", true, func() {
			w.runStateAction("delete preset", func() (bool, error) { return api.SendPresetDelete(name) })
		})
	})
	e.box.Append(remove)
	e.focus = append(e.focus, focusItem{widget: remove, row: e.row, col: 0, section: "preset-actions", onActivate: func() { remove.Activate() }})
	if assigned {
		hint := gtk.NewLabel("Assign another preset before deleting this one.")
		hint.SetHAlign(gtk.AlignStart)
		hint.SetWrap(true)
		hint.AddCSSClass("card-sub")
		e.box.Append(hint)
	}
	changed()

	w.presetDetailFocusItems = append([]focusItem{
		{widget: w.presetDetailBackBtn, row: 0, col: 0, section: "preset-detail-header", onActivate: func() { w.presetDetailBackBtn.Activate() }},
	}, e.focus...)
}
