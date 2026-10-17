#!/bin/sh
# skills/*/src の Go をアーキテクチャ別にビルドし、skills/*/bin/<名前>-<os>-<arch> に置く。
# bin/<名前> は OS とアーキテクチャを見て実体を呼ぶラッパー（scripts/dispatch.sh のコピー）。
set -eu

ROOT=$(cd "$(dirname "$0")/.." && pwd)
TARGETS="darwin/arm64 linux/amd64 linux/arm64"

for skill in zoom-img:zoom_img compare-imgs:compare_imgs; do
	dir=${skill%%:*}
	name=${skill##*:}
	src="$ROOT/skills/$dir/src"
	bin="$ROOT/skills/$dir/bin"
	mkdir -p "$bin"
	for t in $TARGETS; do
		os=${t%/*}
		arch=${t#*/}
		echo "build $name $os/$arch"
		(cd "$src" && CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags="-s -w" -o "$bin/$name-$os-$arch" .)
	done
	cp "$ROOT/scripts/dispatch.sh" "$bin/$name"
	chmod +x "$bin/$name"
done
