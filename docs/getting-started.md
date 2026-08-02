# Quick Start

## Opening the drawer

Press the **Armoury Crate button** on your Z13. The drawer slides in from the
right edge of the screen.

Press it again, click anywhere outside the drawer, or press **Escape** to close it.

---

## Drawer controls

| Section | What it does |
|---------|-------------|
| **Overview** | Monitor CPU, GPU, NPU, memory, battery, power, clocks, temperatures, and fan speed. |
| **Profile** | Switch between quiet, balanced, and performance hardware profiles. |
| **Power Automation** | Assign named presets to plugged-in and battery operation. |
| **CPU Power** | Set the CPU minimum frequency, AMD energy performance preference (EPP), and CPU boost without changing GPU clocks. |
| **Power Tuning** | Open independent TDP, fan-curve, and undervolt overrides. |
| **Fan Mode** | Select firmware Auto or the Balanced and Turbo curve presets. |
| **Battery Limit** | Set the charge cap (40–100%). Changes persist across reboots. |
| **Refresh Rate** | Switch the internal display between 60 Hz and 180 Hz. |
| **Keyboard / Lightbar** | Tab between the two lighting zones |
| **Mode** | Lighting effect: static, breathe, cycle, rainbow, strobe, or off |
| **Color 1 / Color 2** | Pick from 8 presets or open the custom color picker |
| **Speed** | Animation speed for modes that support it: slow, normal, fast |
| **Brightness** | Lighting brightness: 0–3 |
| **Panel Overdrive** | Toggle faster pixel response (may cause slight ghosting) |
| **Boot Sound** | Enable or disable the startup POST sound |

Changes take effect immediately and are sent to the z13ctl-plus daemon. Saved
device settings persist across reboots; CPU power controls reflect live kernel
state.

The theme picker button at the bottom-left of the drawer opens the theme view.
See [Theming](theming.md) for details.

---

## Custom color picker

Click **Custom** under any color input to open the HSL color picker. Adjust
the Hue, Saturation, and Lightness sliders to dial in any color. The preview
swatch updates in real time.

---

## Gamepad navigation

Z13GUI+ supports full gamepad control for use in Steam Gaming Mode:

| Input | Action |
|-------|--------|
| D-pad | Navigate between controls |
| A (Cross) | Activate buttons/switches, or enter edit mode for sliders |
| Left/Right (in edit mode) | Adjust a slider value |
| A (in edit mode) | Commit the value |
| B (Circle) | Cancel edit, go back, or close the drawer |
| L1/R1 (shoulder) | Jump between sections |

Gamepad focus (indicated by a highlight border) is automatically hidden when
the mouse moves. To disable gamepad input entirely, set
`Z13GUI_PLUS_NO_GAMEPAD=1`.

---

## Gamescope (Steam Gaming Mode)

In Steam Gaming Mode, Z13GUI+ runs as a gamescope X11 overlay. The backend is
selected automatically when `GAMESCOPE_WAYLAND_DISPLAY` is set and its socket
is present.

The UI scales automatically to match the output resolution. Use
`Z13GUI_PLUS_SCALE` to override the auto-detected scale factor if the UI appears
too large or small.

Theme and color controls use full-view alternatives on every backend because
gamescope does not composite separate popup windows.
