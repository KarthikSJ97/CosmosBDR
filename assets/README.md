# Brand assets

| File | Use |
|------|-----|
| `logo.svg` | README and docs (self-contained raster embedded at 590×110) |
| `logo.png` | Source/export; same artwork as the SVG |

Regenerate `logo.svg` from `logo.png` after art changes:

```bash
python3 - <<'PY'
import base64
from pathlib import Path
png = Path("assets/logo.png").read_bytes()
b64 = base64.b64encode(png).decode()
svg = Path("assets/logo.svg")
svg.write_text(
    '<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" '
    'width="590" height="110" viewBox="0 0 590 110" role="img" '
    'aria-label="CosmosBDR — Agentless backup. Immutable copies. One-call restoration.">'
    f'<image width="590" height="110" xlink:href="data:image/png;base64,{b64}"/></svg>'
)
PY
```

Terminal startup uses text styling in `src/branding/startup_banner.go` (Docker/log collectors are rarely TTYs).