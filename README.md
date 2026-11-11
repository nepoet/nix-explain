# nix-explain

Explain Nix error messages. Offline, deterministic, no API keys.

```
$ nix build .#foo 2>&1 | nix-explain

Infinite recursion while evaluating
────────────────────────────────────────────────────────────────────────
eval · eval-infinite-recursion

CAUSE
  Nix tried to compute a value in terms of itself. The usual offenders are
  an option whose value mentions the same option (directly or through
  `config`), a module that ends up importing itself, or a `rec` attribute
  set with no base case.

FIX
  Look at the innermost frame that points at your own file (see TRACE).
  Check for `foo = config.foo;`, for a `lib.mkIf` whose condition reads
  the option it guards, and for an `imports` cycle. Then replace the self
  reference with the raw value, or split the definition with
  `lib.mkDefault` / `lib.mkForce`.

TRACE
  → /home/user/nixos/configuration.nix:42:9
    … while evaluating the file '/home/user/nixos/configuration.nix':
  (14 frames from nixpkgs hidden — use --show-all to see them)
```

## Why

Nix error output is a stack trace where the one frame that matters — the
line in *your* file — is buried under dozens of frames from nixpkgs. Most
people solve this by pasting the output into a search engine or a chat
assistant. `nix-explain` does the two useful things locally:

1. **Explains the error.** A curated knowledge base maps messages to a
   cause and a concrete fix.
2. **Finds your frame.** Frames from `/nix/store` are filtered out, leaving
   the files you can actually open.

It never sends your code or logs anywhere.

## Install

```sh
go install github.com/nepoet/nix-explain@latest
```

From a checkout:

```sh
git clone https://github.com/nepoet/nix-explain
cd nix-explain
go build -o nix-explain .
```

## Usage

```sh
# pipe an error into it
nix build .#foo 2>&1 | nix-explain

# or let it run the command (exit code is preserved, output is streamed)
nix-explain run -- nixos-rebuild switch

# explain a message you already have
nix-explain "error: infinite recursion encountered"

# machine readable
nix-explain --json < error.log

# browse the knowledge base
nix-explain list
nix-explain list flake
```

Exit codes: `0` when the output was explained, `2` when nothing matched.
In `run` mode the exit code of the wrapped command is returned unchanged.

## How it works

```
stdin / command output
      │
      ├── normalise: strip ANSI, collapse whitespace
      ├── match:    38 substring rules → best scored hit
      └── trace:    parse `at file:line:col`, drop /nix/store frames
```

The knowledge base is YAML embedded into the binary, so the tool has no
runtime dependencies and works with no network. A rule looks like this:

```yaml
id: eval-infinite-recursion
title: Infinite recursion while evaluating
category: eval
match:
  - "infinite recursion encountered"
cause: |
  Nix tried to compute a value in terms of itself. ...
fix: |
  Look at the innermost frame that points at your own file. ...
see_also:
  - https://nixos.org/manual/nixos/stable/#sec-option-definitions
```

`match` patterns are case-insensitive substrings. `match_all` can be used
when several fragments must appear together. Matching is deliberately
substring-based: Nix rewords messages between versions, so exact matching
would break on every release.

## Coverage

Rules are grouped by the stage where the error happens:

| Category   | Examples                                                        |
|------------|-----------------------------------------------------------------|
| `eval`     | infinite recursion, undefined variable, type coercion, syntax   |
| `modules`  | option does not exist, type mismatch, conflicting definitions    |
| `packages` | unfree, broken, insecure, unsupported platform, collisions       |
| `build`    | builder failed, missing header, pkg-config                       |
| `store`    | hash mismatch, no space, permissions, locks                      |
| `flakes`   | experimental features, missing attributes, dirty git tree        |
| `network`  | substituter unreachable, TLS/proxy issues                        |
| `daemon`   | daemon not running, untrusted user                               |

## Contributing

The most valuable contribution is a rule for an error you hit yourself:

1. Copy the real output into `testdata/cases/<slug>.txt`.
2. Put the expected rule id into `testdata/cases/<slug>.want`.
3. Add a rule to the matching file in `rules/`.
4. Run `go test ./...`.

If a rule matches too eagerly, add a `match_all` entry or raise `priority`
on the more specific rule.

Live end-to-end tests (require Nix):

```sh
go build -o nix-explain .
scripts/e2e.sh ./nix-explain
```

## Limitations

- It is not a general search engine: only errors present in `rules/` are
  recognised. Unmatched output is reported and can be added with a PR.
- Build logs are only triaged at the message level. For full logs use
  `nix log` and `nix-output-monitor`.
- The knowledge base is written in English; a translation layer is not
  implemented yet.

## License

MIT
