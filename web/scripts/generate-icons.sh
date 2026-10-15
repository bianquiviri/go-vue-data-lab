#!/usr/bin/env bash
# Generates the PWA/installable icon set from web/public/icon.svg.
#
# Usage:
#     ./web/scripts/generate-icons.sh
#
# Requires macOS `sips` (built-in). For cross-platform generation prefer
# `npx @vite-pwa/assets-generator` or ImageMagick/rsvg-convert.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PUBLIC_DIR="${SCRIPT_DIR}/../public"
SRC="${PUBLIC_DIR}/icon.svg"

if [[ ! -f "$SRC" ]]; then
	echo "error: source icon not found: $SRC" >&2
	exit 1
fi

# Rasterize the SVG once at its intrinsic size (512x512).
TMP_PNG="$(mktemp -t icon).png"
sips -s format png "$SRC" --out "$TMP_PNG" >/dev/null

resize() {
	local size="$1" out="$2"
	sips -z "$size" "$size" "$TMP_PNG" --out "${PUBLIC_DIR}/${out}" >/dev/null
	echo "[icons] wrote ${out} (${size}x${size})"
}

resize 16  favicon-16x16.png
resize 32  favicon-32x32.png
resize 180 apple-touch-icon.png
resize 192 icon-192x192.png
resize 512 icon-512x512.png

# Maskable icon: same artwork with a safe-zone margin (Android adaptive icons).
# We render the base at 80% and pad to 512 with the theme background color.
MASKABLE_TMP="$(mktemp -t maskable).png"
sips -z 410 410 "$TMP_PNG" --out "$MASKABLE_TMP" >/dev/null
sips -p 512 512 --padColor 0F172A "$MASKABLE_TMP" --out "${PUBLIC_DIR}/icon-maskable-512x512.png" >/dev/null
echo "[icons] wrote icon-maskable-512x512.png (512x512, padded)"

# Multi-resolution favicon.ico (16/32/48) via sips is not supported; use
# ImageMagick if you need a real .ico:
#     magick favicon-16x16.png favicon-32x32.png favicon.ico
echo "[icons] done. Note: favicon.ico requires ImageMagick (optional)."

rm -f "$TMP_PNG" "$MASKABLE_TMP"
