#!/bin/sh
# Installs or updates lazydev from a GitHub release of domsen123/lazydev.
#
#   curl -fsSL https://raw.githubusercontent.com/domsen123/lazydev/main/install.sh | sh
#   sh install.sh v0.2.0
#
# Without argument the newest release. PREFIX defaults to $HOME/.local, the binary lands in $PREFIX/bin.
set -eu

repo=domsen123/lazydev
prefix=${PREFIX:-$HOME/.local}

fail() {
	echo "install.sh: $*" >&2
	exit 1
}

for tool in curl tar sha256sum uname; do
	command -v "$tool" >/dev/null 2>&1 || fail "$tool not found"
done

os=$(uname -s)
[ "$os" = Linux ] || fail "lazydev runs on Linux only"
machine=$(uname -m)
case "$machine" in
x86_64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) fail "unsupported architecture $machine" ;;
esac

if [ $# -gt 0 ]; then
	tag=$1
else
	tag=$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" |
		sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)
	[ -n "$tag" ] || fail "no release found"
fi
version=${tag#v}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
trap 'exit 1' HUP INT TERM

archive=lazydev_${version}_linux_$arch.tar.gz
base=https://github.com/$repo/releases/download/$tag
curl -fsSL -o "$tmp/$archive" "$base/$archive" || fail "download of $archive failed"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || fail "download of checksums.txt failed"

cd "$tmp"
grep " $archive\$" checksums.txt | sha256sum -c - >/dev/null 2>&1 || fail "checksum mismatch"
tar -xzf "$archive" lazydev

mkdir -p "$prefix/bin"
install -m 755 lazydev "$prefix/bin/lazydev"

echo "Installed lazydev $tag to $prefix/bin/lazydev. Restart lazydev to use it."
case ":$PATH:" in
*":$prefix/bin:"*) ;;
*) echo "Add $prefix/bin to PATH." ;;
esac
