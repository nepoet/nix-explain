#!/bin/sh
# nix-explain installer.
#
#   curl -fsSL https://raw.githubusercontent.com/nepoet/nix-explain/main/install.sh | sh
#
# Options (pass after `sh -s --` when piping):
#   --prefix DIR     install to DIR (default: ~/.local/bin)
#   --system         install the binary for all users (needs sudo)
#   --no-shims       do not install nix/nixos-rebuild wrappers
#   --uninstall      remove the binary and its shims
#   --version V      install a specific version (default: pinned below)

set -eu

REPO="nepoet/nix-explain"
VERSION="${NIX_EXPLAIN_VERSION:-0.2.4-hotfix}"

PREFIX="${HOME}/.local/bin"
BINDIR=""
WITH_SHIMS=1
SYSTEM=0
UNINSTALL=0
MODIFY_PATH=1

while [ $# -gt 0 ]; do
	case "$1" in
	--prefix) PREFIX="$2"; shift 2 ;;
	--system) SYSTEM=1; shift ;;
	--no-shims) WITH_SHIMS=0; shift ;;
	--no-modify-path) MODIFY_PATH=0; shift ;;
	--uninstall) UNINSTALL=1; shift ;;
	--version) VERSION="$2"; shift 2 ;;
	-h | --help)
		sed -n '2,14p' "$0" | sed 's/^# \{0,1\}//'
		exit 0
		;;
	*)
		echo "unknown option: $1" >&2
		exit 1
		;;
	esac
done

say() { printf '%s\n' "$*"; }
die() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

SUDO=""
if [ "$SYSTEM" = 1 ]; then
	PREFIX="/usr/local/bin"
	BINDIR="/usr/local/lib/nix-explain"
	command -v sudo >/dev/null 2>&1 || die "--system needs sudo"
	SUDO="sudo"
fi
[ -n "$BINDIR" ] || BINDIR="$PREFIX"

if command -v curl >/dev/null 2>&1; then
	fetch() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -qO "$2" "$1"; }
else
	die "curl or wget is required"
fi

if [ "$UNINSTALL" = 1 ]; then
	for c in nix nixos-rebuild home-manager nix-shell nix-build; do
		for dir in "$PREFIX" "$HOME/.local/bin" /usr/local/bin; do
			[ -L "$dir/$c" ] || continue
			target=$(readlink "$dir/$c")
			case "$target" in
			*nix-explain*) $SUDO rm -f "$dir/$c" ;;
			esac
		done
	done
	$SUDO rm -f "$BINDIR/nix-explain"
	say "removed nix-explain"
	say "captured errors stay in ${XDG_RUNTIME_DIR:-/tmp}/nix-explain"
	exit 0
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
x86_64 | amd64) ARCH=amd64 ;;
aarch64 | arm64) ARCH=arm64 ;;
*) die "unsupported architecture: $ARCH" ;;
esac
case "$OS" in
linux | darwin) ;;
*) die "unsupported OS: $OS (use go install or nix instead)" ;;
esac

TMP=$(mktemp -d 2>/dev/null || mktemp -d -t nix-explain)
trap 'rm -rf "$TMP"' EXIT

ASSET="nix-explain_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE="https://github.com/${REPO}/releases/download/v${VERSION}"

say "downloading ${ASSET}..."
fetch "${BASE}/${ASSET}" "$TMP/pkg.tar.gz" || die "cannot download ${ASSET}"
fetch "${BASE}/checksums.txt" "$TMP/checksums.txt" || die "cannot download checksums.txt"
sed 's#  \./#  #' "$TMP/checksums.txt" > "$TMP/checksums.clean"
mv "$TMP/checksums.clean" "$TMP/checksums.txt"

line=$(grep " ${ASSET}\$" "$TMP/checksums.txt" || true)
[ -n "$line" ] || die "no checksum for ${ASSET}"
expected=${line%% *}
if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "$TMP/pkg.tar.gz" | awk '{print $1}')
else
	actual=$(shasum -a 256 "$TMP/pkg.tar.gz" | awk '{print $1}')
fi
[ "$expected" = "$actual" ] || die "checksum mismatch"

tar -xzf "$TMP/pkg.tar.gz" -C "$TMP"
[ -x "$TMP/nix-explain" ] || die "archive does not contain nix-explain"

$SUDO mkdir -p "$BINDIR"
$SUDO install -m 0755 "$TMP/nix-explain" "$BINDIR/nix-explain"
[ "$BINDIR" = "$PREFIX" ] || $SUDO mkdir -p "$PREFIX"

if [ "$WITH_SHIMS" = 1 ]; then
	for c in nix nixos-rebuild home-manager nix-shell nix-build; do
		$SUDO ln -sf "$BINDIR/nix-explain" "$PREFIX/$c"
	done
fi

say ""
"$BINDIR/nix-explain" --version 2>/dev/null || true
say ""
case ":${PATH}:" in
*":${PREFIX}:"*) ;;
*)
	if [ "$MODIFY_PATH" = 1 ]; then
		added=0
		for rc in "$HOME/.bashrc" "$HOME/.zshrc"; do
			[ -f "$rc" ] || continue
			grep -q "added by nix-explain" "$rc" 2>/dev/null && continue
			{
				printf '\n# added by nix-explain\n'
				printf 'export PATH="%s:$PATH"\n' "$PREFIX"
			} >>"$rc"
			added=1
		done
		if [ "$added" = 1 ]; then
			say "Added ${PREFIX} to PATH in your shell config."
			say "Restart the shell, or run: source ~/.bashrc"
		else
			say "Add ${PREFIX} to PATH manually:"
			say "  export PATH=\"${PREFIX}:\$PATH\""
		fi
	else
		say "Add ${PREFIX} to PATH, for example:"
		say "  echo 'export PATH=\"${PREFIX}:\$PATH\"' >> ~/.bashrc && source ~/.bashrc"
	fi
	;;
esac
say "Done. Run a failing nix command, then just type: nix-explain"
say "Disable the wrappers with NIX_EXPLAIN=0; uninstall with --uninstall."
