#!/bin/sh
# ../libexec/<自分の名前>-<os>-<arch> を呼ぶ。scripts/build.sh が bin/ にコピーする。
set -eu

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case $(uname -m) in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) arch=$(uname -m) ;;
esac

self=$0
while [ -L "$self" ]; do
	link=$(readlink "$self")
	case $link in
		/*) self=$link ;;
		*) self=$(dirname "$self")/$link ;;
	esac
done

name=$(basename "$self")
exe="$(dirname "$self")/../libexec/$name-$os-$arch"
if [ ! -x "$exe" ]; then
	echo "$name: $os/$arch 用のバイナリがありません（$exe）" >&2
	exit 1
fi
exec "$exe" "$@"
