#!/usr/bin/env bash
#
# Live end-to-end test: reproduces real Nix errors and checks that
# nix-explain recognises each of them. Requires nix on PATH.
#
# Usage:
#   scripts/e2e.sh [path-to-binary]
#
# Set FULL=1 to include the network-dependent case.

set -uo pipefail

BIN=${1:-./nix-explain}
FULL=${FULL:-0}

case "$BIN" in
	/*) ;;
	*) BIN="$PWD/${BIN#./}" ;;
esac

if [ ! -x "$BIN" ]; then
	echo "e2e: binary not found: $BIN" >&2
	exit 1
fi
if ! command -v nix >/dev/null 2>&1; then
	echo "e2e: nix not found on PATH" >&2
	exit 1
fi

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
cd "$WORK"

pass=0
fail=0

ok() { pass=$((pass + 1)); printf '  ok   %s\n' "$1"; }
no() { fail=$((fail + 1)); printf '  FAIL %s\n         %s\n' "$1" "$2"; }

# check NAME WANT -- COMMAND...
check() {
	local name=$1 want=$2
	shift 3
	local out rc got
	out=$("$@" 2>&1)
	rc=$?
	if [ "$rc" -eq 0 ]; then
		no "$name" "command succeeded, expected an error"
		return
	fi
	got=$(printf '%s\n' "$out" | "$BIN" --json --color never 2>/dev/null |
		sed -n 's/^ *"id": "\([^"]*\)".*/\1/p' | head -1)
	if [ "$got" = "$want" ]; then
		ok "$name ($want)"
	else
		no "$name" "matched '${got:-<none>}', want '$want'"
	fi
}

echo "== fixtures =="
printf '{ a = 1\n' >syntax.nix
cat >recursion.nix <<'EOF'
let
  a = b;
  b = a;
in a
EOF
printf '{ a = 1; }.b\n' >missing-attr.nix
printf 'builtins.toString { a = 1; }\n' >coerce.nix
printf '({ a }: a) 1\n' >arity.nix
printf '1 + "x"\n' >add-types.nix
printf 'foo + 1\n' >undefined.nix
printf 'let x = 1; x = 2; in x\n' >already-defined.nix

cat >module.nix <<'EOF'
let
  nixpkgs = builtins.getFlake "nixpkgs";
  lib = nixpkgs.legacyPackages.${builtins.currentSystem}.lib;
in (lib.evalModules {
  modules = [
    { options.x = lib.mkOption { type = lib.types.int; }; }
    { config.x = 1; }
    { config.x = 2; }
  ];
}).config.x
EOF

cat >module-type.nix <<'EOF'
let
  nixpkgs = builtins.getFlake "nixpkgs";
  lib = nixpkgs.legacyPackages.${builtins.currentSystem}.lib;
in (lib.evalModules {
  modules = [{
    options.foo = lib.mkOption { type = lib.types.int; };
    config.foo = "not an int";
  }];
}).config.foo
EOF

mkdir -p flake-demo
printf '{ outputs = { self }: { }; }\n' >flake-demo/flake.nix

echo "== evaluation errors =="
check syntax eval-syntax-error -- nix eval --file syntax.nix
check recursion eval-infinite-recursion -- nix eval --file recursion.nix
check missing-attribute eval-attribute-missing -- nix eval --file missing-attr.nix
check coerce eval-cannot-coerce -- nix eval --file coerce.nix
check expected-type eval-expected-type -- nix eval --file arity.nix
check cannot-add eval-cannot-add -- nix eval --file add-types.nix
check undefined-variable eval-undefined-variable -- nix eval --file undefined.nix
check already-defined eval-attribute-already-defined -- nix eval --file already-defined.nix

echo "== module system =="
check module-conflict module-conflict -- nix eval --impure --file module.nix
check module-type module-type-mismatch -- nix eval --impure --file module-type.nix

echo "== flakes =="
check flake-attribute flake-attribute-missing -- nix build ./flake-demo#nope
check feature-disabled flake-feature-disabled -- env NIX_CONFIG='experimental-features =' nix flake show ./flake-demo

echo "== builds =="
check builder-failed build-failed -- nix build --impure --expr \
	'let pkgs = (builtins.getFlake "nixpkgs").legacyPackages.${builtins.currentSystem}; in pkgs.runCommand "boom" {} "echo boom; exit 3"'
check unfree pkg-unfree -- nix build nixpkgs#google-chrome

if [ "$FULL" = "1" ]; then
	echo "== network =="
	check hash-mismatch store-hash-mismatch -- nix build --impure --expr \
		'builtins.fetchurl { url = "https://raw.githubusercontent.com/NixOS/nix/master/README.md"; sha256 = "0000000000000000000000000000000000000000000000000000"; }'
fi

echo "== run mode =="
"$BIN" run --lang en --color never -- nix eval --file missing-attr.nix >run.out 2>run.err
rc=$?
if [ "$rc" -eq 1 ] && grep -q "Attribute missing" run.err; then
	ok "run preserves exit code and explains on stderr"
else
	no "run mode" "exit=$rc, stderr did not contain the explanation"
fi

echo "== shim capture =="
mkdir -p shim/bin shim/real run
ln -sf "$BIN" shim/bin/nix
cat > shim/real/nix <<'SH'
#!/bin/sh
echo "error: infinite recursion encountered" >&2
echo "       at /tmp/rec.nix:2:7:" >&2
exit 1
SH
chmod +x shim/real/nix
XDG_RUNTIME_DIR="$WORK/run" PATH="$WORK/shim/bin:$WORK/shim/real:/usr/bin:/bin" \
	"$WORK/shim/bin/nix" build >/dev/null 2>&1 || true
got=$(XDG_RUNTIME_DIR="$WORK/run" "$BIN" --last --json 2>/dev/null |
	sed -n 's/^ *"id": "\([^"]*\)".*/\1/p' | head -1)
if [ "$got" = "eval-infinite-recursion" ]; then
	ok "shim capture + bare nix-explain"
else
	no "shim capture" "matched '${got:-<none>}'"
fi

echo "== translations =="
nix eval --file missing-attr.nix 2>&1 | "$BIN" --lang ru --color never >ru.out 2>&1
if grep -q "ПРИЧИНА" ru.out && grep -q "ИСПРАВЛЕНИЕ" ru.out; then
	ok "russian output"
else
	no "russian output" "labels were not translated"
fi

echo
printf 'e2e: %d passed, %d failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
