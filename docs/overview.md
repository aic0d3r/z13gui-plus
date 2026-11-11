# Overview

## Interface

The hero on the [home page](index.md) is the **Overview** tab; four more views:

<table>
  <tr>
    <td align="center"><strong>Power</strong><br><img src="../assets/z13gui-plus-power.avif" width="320" alt="Power tab: profile, fan mode, CPU Boost, automation, battery strategy, and tuning cards"></td>
    <td align="center"><strong>RGB</strong><br><img src="../assets/z13gui-plus-rgb.avif" width="320" alt="RGB tab: per-device lighting toggle for keyboard and lightbar"></td>
  </tr>
  <tr>
    <td align="center"><strong>Power Tuning</strong><br><img src="../assets/z13gui-plus-power-tuning.avif" width="320" alt="Power Tuning view: TDP sliders for PL1, PL2, and PL3 with a high-TDP caution"></td>
    <td align="center"><strong>Settings</strong><br><img src="../assets/z13gui-plus-system.avif" width="320" alt="Settings tab: display, camera, boot sound, theme, and layout cards"></td>
  </tr>
</table>

Tabs, roughly in the order you'll use them:

- **Overview** — live temperatures, power draw, fans, memory, and battery
- **Power** — profile, fan mode, CPU Boost, automation, battery strategy
- **Power tuning** — TDP, fan curve, undervolt, and CPU frequency limits
- **RGB** — keyboard and lightbar lighting
- **Settings** — refresh rate, overdrive, boot sound, Dim, camera, theme, layout

## Compatibility

The drawer shows only what the [`z13ctl-plus`](https://github.com/aic0d3r/z13ctl-plus)
daemon reports for your machine. Cards for missing hardware are hidden
automatically, and **Settings → Layout** lets you hide any other card.

| Machine | Status |
|---------|--------|
| ASUS ROG Flow Z13 (2025, GZ302) | **Supported** — every control |
| Other AMD Strix Halo laptop or mini PC | **Experimental** — telemetry and CPU policy should work; fan curves, TDP, boot sound and panel overdrive need ASUS interfaces and are hidden when absent |
| Anything else | Not supported |

Without a battery, the Battery cards disappear; without RGB devices, the RGB tab
disappears; without an internal panel, the refresh-rate control disappears. A
daemon older than v2.5.0 does not report capabilities, and the drawer then shows
everything.

Running it on a non-Z13 machine? Please open a
[compatibility report](https://github.com/aic0d3r/z13gui-plus/issues/new?template=compatibility_report.md)
so this table can be filled in from real results.

## Tablet stack

Z13GUI+ is the settings and status surface in a three-repo tablet stack for the
Z13. Install the other two repos alongside it:

- [z13-tablet-kit](https://github.com/aic0d3r/z13-tablet-kit) — detects
  dock/folio/tablet posture, toggles the touchscreen, and provides touch
  scrolling and two-finger context menus
- [z13ctl-plus](https://github.com/aic0d3r/z13ctl-plus) — owns internal-panel
  scale/rotation/refresh changes and persists tablet settings (desktop touch
  policy, two-finger hold, folio touchpad opt-in, scroll speed, keep display scale)
- **Z13GUI+** (this repo) — GTK4 overlay drawer exposing live posture/health and
  the tablet toggles

Flow: kit detects posture → the daemon applies display changes → this drawer
shows state and persists settings → the kit applies them live — no restarts
required. The tablet chip in this drawer stays hidden until the kit's
`z13-tablet-switch` reports a heartbeat.

## Darker screen

[z13-backlight-quirk](https://github.com/aic0d3r/z13-backlight-quirk) (AUR
`z13-backlight-quirk-dkms`) lowers the Z13's minimum brightness with a kernel
backlight quirk. Without it, **Settings → Display → Dim** darkens the screen in
software instead.
