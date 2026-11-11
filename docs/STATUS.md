# Project status

Short, factual snapshot for anyone (human or agent) picking this up in a new
session.

## What it is

`nix-explain` explains Nix error messages offline. 53 rules, English and
Russian, one static binary. It never sends logs anywhere.

Current release: **v0.2.4-hotfix** (`github.com/nepoet/nix-explain`).

## What works

- `nix-explain "<text>"`, `nix-explain file.log`, `... | nix-explain` — explain
  text, a file or stdin.
- Bare `nix-explain` in a terminal — explains the last error captured for that
  terminal.
- `--last`, `--clipboard`, interactive paste fallback.
- `nix-explain run -- <cmd>` — stream output, keep the exit code, explain after
  failure, save the capture.
- `list`, `--json`, `--lang en|ru|auto`, `--width N`, `--show-all`.
- Flags are accepted before or after the file argument.

## Capture and shims

The installer puts symlinks `nix`, `nixos-rebuild`, `home-manager`, `nix-shell`
and `nix-build` into `~/.local/bin`. Each shim resolves the real binary, passes
output through unchanged, and saves stderr on failure to
`$XDG_RUNTIME_DIR/nix-explain/<pts>.log` (0600, 30 minute freshness, per TTY).
`SUDO_UID` redirects the capture to the invoking user's runtime dir.

`NIX_EXPLAIN=0` bypasses the shim; `NIX_EXPLAIN_HINT=0` hides the hint.

## Layout

```
main.go                 embed rules+i18n, dispatch shim by argv[0]
internal/cli            flags, explain/run/list, clipboard, paste
internal/kb             rule loading, matching, translations
internal/render         terminal output, width handling
internal/trace          `at file:line:col` parsing, /nix/store filtering
internal/state          last-error capture per TTY
internal/shim           wrappers for nix commands
internal/istty          termios-based TTY check (/dev/null is not a TTY)
rules/                  YAML rules (eval, modules, packages, build, flakes, parsers, nixos)
i18n/ru.yaml            Russian translations
testdata/cases          golden fixtures + expected rule id
```

## Decisions, with reasons

- **Shims, not a daemon.** A daemon cannot see commands the shell starts; the
  capture must happen at the command. No background service is needed.
- **Output is not replaced.** The real Nix output is always shown, plus a hint.
  Auto-replacement with "Enter to see raw" is not implemented yet.
- **Capture at the source.** Terminal scrollback is not readable portably
  (kitty remote control off, no tmux), so the shim writes the capture when the
  command fails.
- **Non-TTY passthrough.** Scripts and CI see unchanged behavior; no prompts.
- **Prerelease tag.** `v0.2.4-hotfix` is a semver prerelease, so
  `go install ...@latest` still resolves to v0.2.2. The installer pins the
  version; README documents it.

## Known limitations

- Only commands that go through the shims are captured. Absolute paths
  (`/usr/bin/nix`) bypass them; `sudo` needs `--system`.
- No auto-replacement of Nix output (see above).
- 30 minute capture freshness; commands with non-TTY stderr are not prompted.
- Build logs are triaged at message level only; full logs still need `nix log`.

## Testing

```sh
go test ./...
go vet ./...
go build -o nix-explain .
scripts/e2e.sh ./nix-explain     # requires nix; 17 checks
```

A new rule needs three things: a real output sample in
`testdata/cases/<slug>.txt`, the expected rule id in `<slug>.want`, and the rule
itself. `go test ./...` validates structure, duplicates and translation
completeness.

## Next steps

- CHANGELOG + this file are the handoff; keep them current.
- Auto-replacement mode (show ours, Enter for raw) — needs a careful TTY design.
- AUR package and nixpkgs `pkgs/by-name`.
- More build-log rules (configure, linker, pkg-config already exist; tests,
  missing commands).
- Tester feedback: ask for raw outputs that are not recognised.
