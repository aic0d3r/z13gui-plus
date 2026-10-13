#!/usr/bin/env python3
"""Generate the README lockup and the social-preview banner.

    pip install fonttools && python3 assets/gen_logo.py

Writes:
    logo.svg   README / landing lockup, dark card (scales to any width)
    og.png     1200x630 PNG for GitHub Settings > Social preview and the docs
               og:image (PNG is required there; GitHub rejects SVG)

Both embed the mark (icon.svg) and the "Z13GUI+" wordmark as path data from
internal/gui/fonts/Inter-Bold.ttf. Paths instead of <text> because a README or
share image cannot load webfonts, and text drawn with whatever font the viewer
happens to have is not a logo. Geometry comes from real font metrics, so copy
changes need no hand edits. Needs rsvg-convert (librsvg) for the PNG.
"""
import re
import shutil
import subprocess
from pathlib import Path

from fontTools.pens.boundsPen import BoundsPen
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.ttLib import TTFont

HERE = Path(__file__).resolve().parent
FONT = HERE.parent / "internal/gui/fonts/Inter-Bold.ttf"
ICON = HERE / "icon.svg"
LOGO_OUT = HERE / "logo.svg"
OG_OUT = HERE / "og.png"

# Tokens from internal/gui/theme-default.toml (rog-dark).
BG = "#1a1a1a"
BORDER = "#3d3d3d"
TEXT = "#e0e0e0"
DIM = "#9a9a9a"  # lighter than theme text_dim: used for small tracked type
ACCENT = "#cc0000"

WORD = "Z13GUI"
PLUS = "+"
WORD_SIZE = 76
TRACK = -0.012  # tracking as a fraction of size; floor is -0.04em
# Inter's plus is a math glyph: small and light next to the caps, so it is scaled
# up and centred on cap height instead of sitting on the baseline.
PLUS_SCALE = 1.22
PLUS_GAP = 6
PAD_X = 34
ICON_BOX = 104
GAP = 26
HEIGHT = 148
CARD_R = 28

# Social preview: GitHub wants 1200x630 and crops nothing, so compose at that size.
OG_W, OG_H = 1200, 630
OG_SCALE = 0.72 * OG_W  # lockup target width
OG_CAPTION = "ASUS ROG FLOW Z13 CONTROL DRAWER"
OG_CAP_SIZE = 33
OG_CAP_TRACK = 0.2
OG_CAP_GAP = 38

FONT_CACHE: dict = {}


def font():
    if not FONT_CACHE:
        f = TTFont(FONT)
        FONT_CACHE["f"] = f
        FONT_CACHE["scale"] = WORD_SIZE / f["head"].unitsPerEm
        FONT_CACHE["cap"] = f["OS/2"].sCapHeight * FONT_CACHE["scale"]
    return FONT_CACHE


def text_paths(text, size, x, baseline, tracking):
    """Path data for `text` at (x, baseline). Returns (paths, width)."""
    cache = font()
    f = cache["f"]
    scale = size / f["head"].unitsPerEm
    cmap, glyphs, hmtx = f.getBestCmap(), f.getGlyphSet(), f["hmtx"]
    paths, cursor = [], float(x)
    for ch in text:
        name = cmap[ord(ch)]
        pen = SVGPathPen(glyphs)
        glyphs[name].draw(pen)
        if d := pen.getCommands():
            paths.append(
                f'<path transform="translate({cursor:.2f} {baseline:.2f})'
                f' scale({scale:.6f} {-scale:.6f})" d="{d}"/>'
            )
        cursor += hmtx[name][0] * scale + tracking * size
    return "".join(paths), cursor - float(x) - tracking * size


def plus_block(x, baseline, cap):
    """The "+" drawn at cap-height centre. Returns (path, width)."""
    f = font()["f"]
    scale = WORD_SIZE * PLUS_SCALE / f["head"].unitsPerEm
    name = f.getBestCmap()[ord(PLUS)]
    glyphs = f.getGlyphSet()
    pen, bounds = SVGPathPen(glyphs), BoundsPen(glyphs)
    glyphs[name].draw(pen)
    glyphs[name].draw(bounds)
    x_min, y_min, x_max, y_max = bounds.bounds
    # transform maps glyph (gx, gy) to (tx + gx*scale, ty - gy*scale)
    tx = x - x_min * scale
    ty = baseline - cap / 2 + (y_min + y_max) / 2 * scale
    path = f'<path transform="translate({tx:.2f} {ty:.2f})'
    path += f' scale({scale:.6f} {-scale:.6f})" d="{pen.getCommands()}"/>'
    return path, (x_max - x_min) * scale


def icon_block(x, y, box):
    """The committed mark, embedded so each output stays a standalone file."""
    src = ICON.read_text()
    defs = src[src.index("<defs>") : src.index("</defs>") + len("</defs>")]
    body = src[src.index("<defs>") : src.rindex("</svg>")]
    body = body[body.index("</defs>") + len("</defs>") :].strip()
    body = re.sub(r"<!--.*?-->", "", body, flags=re.S).strip()
    # unique ids: the mark may appear next to icon.svg on the same page
    for old, new in (('id="z13-', 'id="lg-z13-'), ("url(#z13-", "url(#lg-z13-")):
        defs, body = defs.replace(old, new), body.replace(old, new)
    scale = box / 128
    return defs[6:-7], f'<g transform="translate({x} {y}) scale({scale})">{body}</g>'


def lockup():
    """Mark + wordmark on its own. Returns (defs, body, width, height)."""
    word_paths, word_w = text_paths(WORD, WORD_SIZE, 0, 0, TRACK)
    baseline = round((HEIGHT + font()["cap"]) / 2, 2)  # centre on cap height
    plus_x = PAD_X + ICON_BOX + GAP + word_w + PLUS_GAP
    plus_paths, plus_w = plus_block(plus_x, baseline, font()["cap"])
    word_paths, _ = text_paths(WORD, WORD_SIZE, PAD_X + ICON_BOX + GAP, baseline, TRACK)
    defs, icon = icon_block(PAD_X, round((HEIGHT - ICON_BOX) / 2), ICON_BOX)
    width = round(plus_x + plus_w + PAD_X)
    body = (
        f'{icon}\n<g fill="{TEXT}">{word_paths}</g>\n<g fill="{ACCENT}">{plus_paths}</g>'
    )
    return defs, body, width, HEIGHT


def write_logo(defs, body, width, height):
    svg = f"""<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}"
     viewBox="0 0 {width} {height}" role="img" aria-label="Z13GUI+">
  <!-- GENERATED by assets/gen_logo.py from assets/icon.svg + Inter Bold. Do not edit. -->
  <defs>{defs}</defs>
  <rect x="0.5" y="0.5" width="{width - 1}" height="{height - 1}" rx="{CARD_R}" fill="{BG}"
        stroke="{BORDER}" stroke-width="1"/>
  {body}
</svg>
"""
    LOGO_OUT.write_text(svg)
    print(f"{LOGO_OUT.name} {width}x{height}")


def write_og(defs, body, width, height):
    if not shutil.which("rsvg-convert"):
        print("rsvg-convert not found: wrote logo.svg, skipped og.png")
        return
    scale = OG_SCALE / width
    scaled_w, scaled_h = width * scale, height * scale
    cap_paths, cap_w = text_paths(OG_CAPTION, OG_CAP_SIZE, 0, 0, OG_CAP_TRACK)
    block_h = scaled_h + OG_CAP_GAP + OG_CAP_SIZE
    top = (OG_H - block_h) / 2
    ox, oy = round((OG_W - scaled_w) / 2), round(top)
    cap_x = round((OG_W - cap_w) / 2)
    cap_baseline = round(top + scaled_h + OG_CAP_GAP + OG_CAP_SIZE * 0.78, 2)
    cap_paths, _ = text_paths(OG_CAPTION, OG_CAP_SIZE, cap_x, cap_baseline, OG_CAP_TRACK)

    svg = f"""<svg xmlns="http://www.w3.org/2000/svg" width="{OG_W}" height="{OG_H}"
     viewBox="0 0 {OG_W} {OG_H}" role="img" aria-label="Z13GUI+">
  <defs>{defs}</defs>
  <rect width="{OG_W}" height="{OG_H}" fill="{BG}"/>
  <g transform="translate({ox} {oy}) scale({scale:.6f})">{body}</g>
  <g fill="{DIM}">{cap_paths}</g>
</svg>
"""
    subprocess.run(
        ["rsvg-convert", "-w", str(OG_W), "-h", str(OG_H), "-o", str(OG_OUT)],
        input=svg.encode(),
        check=True,
    )
    print(f"{OG_OUT.name} {OG_W}x{OG_H} ({OG_OUT.stat().st_size // 1024} KB)")


def main():
    defs, body, width, height = lockup()
    write_logo(defs, body, width, height)
    write_og(defs, body, width, height)


if __name__ == "__main__":
    main()
