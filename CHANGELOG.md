# Changelog

## v0.2.4-hotfix — 2026-11-11

- Fix: shims running under `sudo` write the capture to the invoking user's
  runtime directory (`SUDO_UID`), so `nix-explain` finds it afterwards.
- Installer: checksum lookup no longer breaks on the `./` prefix printed by
  `sha256sum`; PATH is added to `~/.bashrc`/`~/.zshrc` automatically
  (idempotent, `--no-modify-path` to opt out).
- Release assets: linux/darwin, amd64/arm64, with `checksums.txt`.

## v0.2.3-hotfix — 2026-11-11

- Bare `nix-explain` explains the last error captured for the current terminal.
- Shims for `nix`, `nixos-rebuild`, `home-manager`, `nix-shell`, `nix-build`.
- `--last`, `--clipboard`, interactive paste fallback.
- Flags accepted before or after the file argument.
- One-line installer with checksum verification.
- `run` mode stores its capture.

## v0.2.2 — 2026-11-11

- New rules: `eval-parse-json` (priority above the generic syntax rule) and
  `eval-parse-toml`.
- Russian translations for both; 53 rules total.

## v0.2.1 — 2026-11-11

- `--width N` for narrow terminals and vertical recordings.
- `list` switches to a two-line layout below 60 columns.
- Rune-aware wrapping so Cyrillic lines break at the same width.

## v0.2.0 — 2026-11-11

- Russian interface and translations for all rules (`--lang en|ru|auto`).
- `flake.nix` with package, app, devShell and formatter.
- 13 new rules (51 total).

## v0.1.0 — 2026-11-11

- First release: CLI, trace pruning, 38 rules, JSON output, `run` mode,
  golden tests, live e2e, CI.
