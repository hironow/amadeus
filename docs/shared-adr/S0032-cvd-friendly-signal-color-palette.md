# S0032. CVD-Friendly Signal Color Palette

**Date:** 2026-03-13
**Status:** Accepted

## Context

4 tools (sightjack, amadeus, paintress, phonewave) share a common `platform.Logger` with ANSI color output. The original palette used bold green (`\033[1;32m`) for OK status, which is indistinguishable from bold red (`\033[1;31m`) for users with protanopia or deuteranopia (red-green color vision deficiency, affecting ~8% of males).

Doctor command output and general logging both rely on these colors to convey status at a glance.

## Decision

Adopt a **signal light palette** (blue, yellow, red) as the base, with gray as a fourth distinct color for non-status information:

| Role | Color | ANSI Code | Signal Analogy |
|------|-------|-----------|----------------|
| OK | Bold Blue | `\033[1;34m` | Go (blue/green light) |
| WARN | Yellow | `\033[33m` | Caution |
| ERR/FAIL | Bold Red | `\033[1;31m` | Stop |
| SKIP/DBUG | Gray | `\033[90m` | N/A (achromatic) |
| INFO | Cyan | `\033[36m` | N/A (blue-green axis) |

Design principles:

- **3-color signal base**: Blue-Yellow-Red provides 3-way discrimination safe for all common CVD types (blue-yellow axis is preserved by protanopia/deuteranopia)
- **4th color is achromatic**: Gray carries no hue, distinguishable purely by brightness regardless of color vision
- **5th color stays on blue axis**: Cyan (INFO) is perceptually close to blue, not confused with red or yellow
- **Non-color redundancy**: Text prefixes (INFO/OK/WARN/ERR/DBUG) always accompany color, ensuring accessibility even with `NO_COLOR=1`
- **Bold weight**: Provides brightness cue independent of hue perception

Reference: Wong (2011), Nature Methods "Points of view: Color blindness"

## Consequences

### Positive

- Red-green CVD users can distinguish all status levels
- Consistent palette across all 4 tools
- `NO_COLOR` and non-terminal detection already respected

### Negative

- Users accustomed to green=OK convention may need brief adjustment
- Bold blue is less "obviously success" than green for color-normal users

### Neutral

- Banner colors (inverted green/cyan for D-MAIL SEND/RECV) are unaffected; they use inverted display mode which provides sufficient visual separation independent of hue
