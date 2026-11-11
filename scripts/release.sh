#!/usr/bin/env bash
# Build release assets and publish them.
#
#   scripts/release.sh            # build only
#   scripts/release.sh --publish  # build and create the GitHub release
set -euo pipefail

VERSION=${VERSION:-0.2.4-hotfix}
TAG="v${VERSION}"
OUT=dist
REPO=nepoet/nix-explain

rm -rf "$OUT"
mkdir -p "$OUT"

for os in linux darwin; do
	for arch in amd64 arm64; do
		echo "building ${os}/${arch}"
		CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
			go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
			-o "$OUT/nix-explain" .
		tar -czf "$OUT/nix-explain_${VERSION}_${os}_${arch}.tar.gz" -C "$OUT" nix-explain
		rm -f "$OUT/nix-explain"
	done
done

(cd "$OUT" && sha256sum ./*.tar.gz | sed 's#  \./#  #' > checksums.txt)
ls -lh "$OUT"

if [ "${1:-}" = "--publish" ]; then
	gh release create "$TAG" "$OUT"/*.tar.gz "$OUT"/checksums.txt \
		--repo "$REPO" \
		--title "${TAG} (hotfix)" \
		--notes "Hotfix: bare \`nix-explain\` now explains the last error of the current terminal.

- shims for nix, nixos-rebuild, home-manager, nix-shell, nix-build capture stderr
- \`nix-explain\` with no arguments explains that capture (or accepts a paste)
- \`--last\` and \`--clipboard\` modes
- flags may follow the file argument
- one-line installer: curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | sh"
fi
