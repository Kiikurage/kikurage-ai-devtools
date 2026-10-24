#!/bin/sh
# skills/*/src の Go をアーキテクチャ別にビルドし、libexec/<名前>-<os>-<arch> に置く。
# bin/<名前> は OS とアーキテクチャを見て実体を呼ぶラッパー（scripts/dispatch.sh のコピー）。
# プラグインの bin/ は Bash ツールの PATH に入るので、Claude は <名前> だけで呼べる。
set -eu

ROOT=$(cd "$(dirname "$0")/.." && pwd)
TARGETS="darwin/arm64 linux/amd64 linux/arm64"

mkdir -p "$ROOT/bin" "$ROOT/libexec"
for skill in zoom-img:zoom_img compare-imgs:compare_imgs; do
	dir=${skill%%:*}
	name=${skill##*:}
	src="$ROOT/skills/$dir/src"
	for t in $TARGETS; do
		os=${t%/*}
		arch=${t#*/}
		echo "build $name $os/$arch"
		(cd "$src" && CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags="-s -w" -o "$ROOT/libexec/$name-$os-$arch" .)
	done
	cp "$ROOT/scripts/dispatch.sh" "$ROOT/bin/$name"
	chmod +x "$ROOT/bin/$name"
done
