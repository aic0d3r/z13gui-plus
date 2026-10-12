---
target: power automation + presets
total_score: 24
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 3
target_identity: "file:/home/ezflow/coding/z13gui-plus/internal/gui/presets.go"
target_fingerprint: "sha256:1ac85854d02c7437dfc20c76ae9054f0803c9a6050aabeb1ac4e6b64edab3979"
target_path: /home/ezflow/coding/z13gui-plus/internal/gui/presets.go
timestamp: 2026-10-12T22-33-33Z
slug: internal-gui-presets-go
---
Method: dual-agent (A: reviewer, B: worker). Target: Power Automation card, Presets screen, preset editor.

Score 24/40 (Acceptable). H1 3, H2 3, H3 2, H4 2, H5 2, H6 3, H7 3, H8 2, H9 2, H10 2.

Specificity: behavior/copy product-specific ("Turning on applies X now", "returns on the next plug change", safety fan copy); visuals are the generic card + btn-group grid. Detector: 0 findings (does not inspect GTK CSS/Go = no signal). Fallback scan: hard-coded rgba in .pill/.pill.success/.pill.warning/.pill.accent (theme-default.css:391-416, accent hard-coded rog red); 9px .preset-card .pill (layout.css:59); icon-only assign toggles presets.go:418 rely on tooltips.

Priority issues:
- [P1] Preset chips don't ellipsize in a 3-column homogeneous grid; long/auto names can widen the 320px drawer (presets.go syncPresetChips). Fix: ellipsize + 2 columns for long names. -> layout
- [P1] High-TDP presets apply from a chip with the warning only in a tooltip; fan lock discovered after the fact. Fix: warning tint + marker on chip, keep one tap. -> harden
- [P1] Pill colors bypass @z13-* tokens (accent stays red on other themes); 9px card pills not in gamescope scaledCSS. Fix: alpha(@z13-accent/warning), >=10px, add to scaledCSS. -> polish
- [P2] Meaning lives in tooltips and a 3s bottom toast (icons, save hint, chip summaries, errors): invisible in touch/gamescope. Fix: text labels on assign toggles, feedback on the tapped chip, sticky errors. -> clarify
- [P2] Dead ends: assigned toggle silent no-op, Update from Current Settings overwrites without confirm, disabled Applied still in gamepad focus, editor Back drops edits, filled Reapply is loudest element in manual mode. -> harden

Personas: Alex (no assign from main card, new preset not scrolled to, empty status gap); Sam (9-10px dim text, 0.55 opacity off-state, color-only assigned state, sliders not Tab-focusable); Kai (no tooltips, 3s toast, Save at end of long editor, Steam OSK for rename unverified).

Minor: case inconsistency (ACTIVE NOW vs Active), "used plugged in and applied" phrasing, dimmed rows look tappable, back button no tooltip, 165-char restore confirm.

Questions: chips with source markers replacing summary rows? Quiet Reapply link instead of filled button? Show unassigned presets on main card at all?
