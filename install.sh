#!/bin/sh
# Installs or updates lazydev from a GitHub release of domsen123/lazydev.
#
#   gh api repos/domsen123/lazydev/contents/install.sh -H 'Accept: application/vnd.github.raw' | sh
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

for tool in gh tar sha256sum uname; do
	command -v "$tool" >/dev/null 2>&1 || fail "$tool not found"
done
gh auth status >/dev/null 2>&1 || fail "gh is not logged in, run gh auth login"

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
	tag=$(gh release view --repo "$repo" --json tagName --jq .tagName)
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
trap 'exit 1' HUP INT TERM

gh release download "$tag" --repo "$repo" --dir "$tmp" \
	--pattern "lazydev_*_linux_$arch.tar.gz" --pattern checksums.txt

cd "$tmp"
grep "_linux_$arch.tar.gz" checksums.txt | sha256sum -c - >/dev/null 2>&1 || fail "checksum mismatch"
tar -xzf lazydev_*_linux_"$arch".tar.gz lazydev

mkdir -p "$prefix/bin"
install -m 755 lazydev "$prefix/bin/lazydev"

echo "Installed lazydev $tag to $prefix/bin/lazydev. Restart lazydev to use it."
case ":$PATH:" in
*":$prefix/bin:"*) ;;
*) echo "Add $prefix/bin to PATH." ;;
esac
