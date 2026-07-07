#!/usr/bin/env bash
#
# Regenerate docs/demo.gif — the README montage: a fast "sizzle reel" that
# glimpses several Flatte sample apps so the reader gets curious, rather than a
# slow feature-by-feature walkthrough.
#
# Requires: go, charmbracelet/vhs (https://github.com/charmbracelet/vhs), ffmpeg.
# Run from anywhere:  ./cmd/record-demo.sh
#
set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$REPO/docs/demo.gif"
WORK="$(mktemp -d)"
BIN="$WORK/bin"; CLIPS="$WORK/clips"
mkdir -p "$BIN" "$CLIPS"
trap 'rm -rf "$WORK"' EXIT

# Shared geometry so the clips concatenate seamlessly.
W=1200; H=720; FS=16
# Order is the cut order. Each app is glimpsed for ~2s; the ones that animate on
# their own (docker logs, progress) carry the motion, Snake supplies the play.
APPS=(flat-game flat-docker flat-workspace flat-style flat-table flat-progress)

for a in "${APPS[@]}"; do
  echo "build $a"; (cd "$REPO/cmd" && go build -o "$BIN/$a" "./$a")
done

clip() { # <name> <body-tape>
  cat > "$CLIPS/$1.tape" <<EOF
Output "$CLIPS/$1.mp4"
Set Shell "bash"
Set FontSize $FS
Set Width $W
Set Height $H
Set Padding 24
Set Theme "Catppuccin Mocha"
Hide
Type "$BIN/$1"  Enter
Sleep 700ms
Show
$2
EOF
  echo "record $1"; vhs "$CLIPS/$1.tape" >/dev/null 2>&1
}

clip flat-game 'Sleep 500ms
Right Sleep 450ms
Down  Sleep 450ms
Left  Sleep 450ms
Up    Sleep 450ms
Right Sleep 700ms'

clip flat-docker 'Sleep 400ms
Tab   Sleep 250ms
Type "l"  Sleep 1500ms'

clip flat-workspace 'Sleep 600ms
Tab   Sleep 450ms
Down  Sleep 350ms
Down  Sleep 650ms'

clip flat-style    'Sleep 1700ms'
clip flat-table    'Sleep 1700ms'
clip flat-progress 'Sleep 2000ms'

# Concatenate the clips, then encode a high-framerate palette-optimized GIF.
LIST="$WORK/list.txt"; : > "$LIST"
for a in "${APPS[@]}"; do echo "file '$CLIPS/$a.mp4'" >> "$LIST"; done
ffmpeg -y -f concat -safe 0 -i "$LIST" -c copy "$WORK/all.mp4" >/dev/null 2>&1

ffmpeg -y -i "$WORK/all.mp4" \
  -vf "fps=30,scale=900:-1:flags=lanczos,palettegen=stats_mode=diff" \
  "$WORK/pal.png" >/dev/null 2>&1
ffmpeg -y -i "$WORK/all.mp4" -i "$WORK/pal.png" \
  -lavfi "fps=30,scale=900:-1:flags=lanczos[x];[x][1:v]paletteuse=dither=bayer:bayer_scale=3" \
  "$OUT" >/dev/null 2>&1

echo "wrote $OUT ($(du -h "$OUT" | cut -f1))"
