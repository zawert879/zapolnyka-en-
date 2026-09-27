#!/bin/sh
# Пересобирает icon.icns из icon.svg. Только macOS: нужны Chrome, sips, iconutil.
# Запускать после правки icon.svg; результат (icon.icns) коммитится.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
chrome="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# Headless-Chrome после снимка сам не выходит — ждём файл и завершаем его.
"$chrome" --headless=new --disable-gpu --hide-scrollbars --user-data-dir="$tmp/profile" \
	--window-size=1024,1024 --default-background-color=00000000 \
	--screenshot="$tmp/icon.png" "file://$here/icon.svg" >/dev/null 2>&1 &
pid=$!
for _ in $(seq 60); do
	[ -s "$tmp/icon.png" ] && break
	sleep 0.5
done
sleep 0.5
kill "$pid" 2>/dev/null || true
wait "$pid" 2>/dev/null || true
[ -s "$tmp/icon.png" ] || { echo "Chrome не отрисовал icon.svg" >&2; exit 1; }

set="$tmp/icon.iconset"
mkdir "$set"
for size in 16 32 128 256 512; do
	sips -z $size $size "$tmp/icon.png" --out "$set/icon_${size}x${size}.png" >/dev/null
	sips -z $((size * 2)) $((size * 2)) "$tmp/icon.png" --out "$set/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$set" -o "$here/icon.icns"
echo "$here/icon.icns"
