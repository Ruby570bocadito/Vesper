#!/usr/bin/env python3
"""Generate docs/images/banner.png from the REAL `vesper version` output.

Renders the actual ANSI output of the CLI into an HTML terminal window
and screenshots it with Playwright — no fake mockups.
"""
import html
import re
import subprocess
import sys
from pathlib import Path

VESPER_BIN = sys.argv[1] if len(sys.argv) > 1 else "/tmp/vesper-test"
REPO_ROOT = Path(__file__).resolve().parent.parent
OUT = REPO_ROOT / "docs" / "images" / "banner.png"

proc = subprocess.run([VESPER_BIN, "version"], capture_output=True, timeout=30)
raw = proc.stdout.decode("utf-8", errors="replace")

# ── ANSI → HTML ──────────────────────────────────────────────────────────────
FG256 = re.compile(r"\033\[38;5;(\d+)m")


def sgr_to_html(line: str) -> str:
    """Convert SGR sequences in one line to styled spans."""
    out = []
    open_span = None

    def close():
        nonlocal open_span
        if open_span:
            out.append("</span>")
            open_span = None

    i = 0
    while i < len(line):
        m = FG256.match(line, i)
        if m:
            close()
            code = int(m.group(1))
            r = (code - 16) // 36 * 51 if code >= 16 else 0
            g = ((code - 16) % 36) // 6 * 51 if code >= 16 else 0
            b = (code - 16) % 6 * 51 if code >= 16 else 0
            if code >= 232:  # grayscale ramp
                v = (code - 232) * 10 + 8
                r = g = b = v
            open_span = f'<span style="color:rgb({r},{g},{b})">'
            out.append(open_span)
            i = m.end()
            continue
        if line.startswith("\033[1m", i):
            close()
            open_span = '<span style="font-weight:bold">'
            out.append(open_span)
            i += 4
            continue
        if line.startswith("\033[3m", i):
            close()
            open_span = '<span style="font-style:italic">'
            out.append(open_span)
            i += 4
            continue
        if line.startswith("\033[0m", i):
            close()
            i += 4
            continue
        out.append(html.escape(line[i]))
        i += 1
    close()
    return "".join(out)


body_lines = [sgr_to_html(l.rstrip("\n")) for l in raw.split("\n")]
body = "\n".join(body_lines)

page = f"""<!DOCTYPE html>
<html><head><meta charset="utf-8"><style>
  body {{ margin:0; background:#0d1117; display:flex; justify-content:center; }}
  .term {{
    background:#0d1117; padding:36px 48px; min-width:760px;
    font-family:'JetBrains Mono','Fira Code','DejaVu Sans Mono',monospace;
    font-size:15px; line-height:1.45; color:#c9d1d9; white-space:pre;
  }}
</style></head>
<body><div class="term" id="term">{body}</div></body></html>"""

tmp = Path("/tmp/vesper_banner.html")
tmp.write_text(page)

from playwright.sync_api import sync_playwright

with sync_playwright() as p:
    browser = p.chromium.launch()
    pg = browser.new_page(viewport={"width": 900, "height": 700}, device_scale_factor=2)
    pg.goto(tmp.as_uri())
    pg.wait_for_timeout(300)
    el = pg.query_selector("#term")
    el.screenshot(path=str(OUT))
    browser.close()

print(f"banner written: {OUT} ({OUT.stat().st_size} bytes)")
