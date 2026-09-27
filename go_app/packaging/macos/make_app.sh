#!/bin/sh
# Собирает Zapolnyaka.app из готового бинаря zapolnyaka-app (darwin) и
# подписывает ad-hoc. Подпись нужна только на macOS (codesign); без неё бандл
# собирается, но скачанный из интернета macOS назовёт «повреждённым».
#
#   make_app.sh <бинарь> <версия> <папка>    → <папка>/Zapolnyaka.app
#
# Локально (из go_app/):
#   go build -o ../dist/zapolnyaka-app ./app
#   packaging/macos/make_app.sh ../dist/zapolnyaka-app dev ../dist
set -eu

[ $# -eq 3 ] || { echo "usage: $0 <бинарь> <версия> <папка>" >&2; exit 2; }
bin=$1
version=${2#v}
out=$3
here=$(cd "$(dirname "$0")" && pwd)

# В Info.plist версия — только числа через точку; у локальной сборки («dev») её нет.
case $version in
	'' | *[!0-9.]*) version=0.0.0 ;;
esac

app="$out/Zapolnyaka.app"
rm -rf "$app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp "$bin" "$app/Contents/MacOS/zapolnyaka-app"
chmod 755 "$app/Contents/MacOS/zapolnyaka-app"
cp "$here/icon.icns" "$app/Contents/Resources/icon.icns"
sed "s/__VERSION__/$version/g" "$here/Info.plist" > "$app/Contents/Info.plist"

# Go подписывает сам бинарь, но не бандл: Info.plist и ресурсы не запечатаны,
# и codesign --verify отвечает «code has no resources but signature indicates
# they must be present». Ad-hoc подпись бандла (без сертификата) это чинит.
if command -v codesign >/dev/null 2>&1; then
	codesign --force --sign - "$app"
else
	echo "warning: codesign не найден — бандл не подписан" >&2
fi
echo "$app"
