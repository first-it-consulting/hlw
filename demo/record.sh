#!/usr/bin/env bash
# Rebuild demo/hlw.gif.
#
#   ./demo/record.sh
#
# Needs python3 with pyte and pillow, plus ffmpeg:
#   python3 -m venv /tmp/hlwdemo && /tmp/hlwdemo/bin/pip install pyte pillow
#   brew install ffmpeg
set -euo pipefail

cd "$(dirname "$0")/.."
PY=${PY:-python3}
$PY -c "import pyte, PIL" 2>/dev/null || {
  echo "need pyte and pillow: pip install pyte pillow" >&2; exit 1
}
command -v ffmpeg >/dev/null || { echo "need ffmpeg" >&2; exit 1; }

go build -o hlw .

# A stub model list keeps the recording reproducible and free of whatever
# models happen to be installed locally.
$PY demo/models-server.py & SERVER=$!
trap 'kill $SERVER 2>/dev/null || true' EXIT
sleep 1

DEMOHOME=$(mktemp -d); mkdir -p "$DEMOHOME/.config/hlw"
cp demo/config.json "$DEMOHOME/.config/hlw/config.json"
FRAMES=$(mktemp -d); BUILD=$(mktemp -d)

ROWS=21 COLS=104 FRAMEDIR="$FRAMES" SCRIPT=demo/script.json \
  DEMOHOME="$DEMOHOME" DEMOPATH="$PWD:$(dirname "$(command -v claude || echo /usr/bin/true)"):/opt/homebrew/bin:/usr/bin:/bin" \
  $PY demo/record.py

# Keep frames up to hlw's hand-off, then hold the last one so it can be read.
$PY - "$FRAMES" "$BUILD" <<'TRIM'
import os, sys, glob, shutil
src, dst = sys.argv[1], sys.argv[2]
cut = 0
for t in sorted(glob.glob(os.path.join(src, "f*.txt"))):
    s = open(t).read()
    if "Launching..." in s and "get started" not in s and "text style" not in s:
        cut = int(os.path.basename(t)[1:6])
n = 0
for i in range(cut + 1):
    p = os.path.join(src, "f%05d.png" % i)
    if os.path.exists(p):
        shutil.copy(p, os.path.join(dst, "o%05d.png" % n)); n += 1
for _ in range(16):
    shutil.copy(os.path.join(src, "f%05d.png" % cut), os.path.join(dst, "o%05d.png" % n)); n += 1
print("frames: %d (cut at %d)" % (n, cut))
TRIM

ffmpeg -v error -framerate 10 -i "$BUILD/o%05d.png" \
  -vf "scale=1000:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=128:stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=4:diff_mode=rectangle" \
  -loop 0 -y demo/hlw.gif

ls -lh demo/hlw.gif
