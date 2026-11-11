// Package i18n holds the user interface strings for every supported
// language. Rule texts live next to the rules (see i18n/<lang>.yaml).
package i18n

import (
	"os"
	"strings"
)

// Strings is the translated user interface.
type Strings struct {
	Lang         string
	Cause        string
	Fix          string
	Example      string
	SeeAlso      string
	Trace        string
	AlsoMatched  string
	NoMatchTitle string
	NoMatchIntro string
	NoMatchList  string
	NoMatchIssue string
	HiddenOne    string
	HiddenMany   string
	Usage        string
	Categories   map[string]string
}

// Resolve maps a --lang value to a supported language. "auto" (or an empty
// value) inspects NIX_EXPLAIN_LANG, LC_ALL, LC_MESSAGES and LANG.
func Resolve(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "en", "ru":
		return strings.ToLower(strings.TrimSpace(lang))
	case "", "auto":
	default:
		return "en"
	}
	for _, key := range []string{"NIX_EXPLAIN_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		v := strings.ToLower(os.Getenv(key))
		if v == "" {
			continue
		}
		if strings.HasPrefix(v, "ru") {
			return "ru"
		}
		return "en"
	}
	return "en"
}

// Get returns the strings for lang, falling back to English.
func Get(lang string) Strings {
	if lang == "ru" {
		return russian
	}
	return english
}

var english = Strings{
	Lang:         "en",
	Cause:        "CAUSE",
	Fix:          "FIX",
	Example:      "EXAMPLE",
	SeeAlso:      "SEE ALSO",
	Trace:        "TRACE",
	AlsoMatched:  "Also matched:",
	NoMatchTitle: "No known explanation for this output.",
	NoMatchIntro: "The error was not recognised. A few things you can do:",
	NoMatchList: "  - %s                 list every known error\n" +
		"  - %s       show the full trace\n" +
		"  - %s  ask Nix for more detail",
	NoMatchIssue: "If this is a common error, please open an issue with the output:",
	HiddenOne:    "(%d frame from nixpkgs hidden — use --show-all to see it)",
	HiddenMany:   "(%d frames from nixpkgs hidden — use --show-all to see them)",
	Categories: map[string]string{
		"eval":     "EVAL",
		"modules":  "MODULES",
		"packages": "PACKAGES",
		"build":    "BUILD",
		"store":    "STORE",
		"flakes":   "FLAKES",
		"network":  "NETWORK",
		"daemon":   "DAEMON",
		"nixos":    "NIXOS",
	},
	Usage: usageEN,
}

var russian = Strings{
	Lang:         "ru",
	Cause:        "ПРИЧИНА",
	Fix:          "ИСПРАВЛЕНИЕ",
	Example:      "ПРИМЕР",
	SeeAlso:      "СМ. ТАКЖЕ",
	Trace:        "ТРАССИРОВКА",
	AlsoMatched:  "Также подошло:",
	NoMatchTitle: "Для этого вывода нет известного объяснения.",
	NoMatchIntro: "Ошибка не распознана. Что можно сделать:",
	NoMatchList: "  - %s                 список известных ошибок\n" +
		"  - %s       показать полную трассировку\n" +
		"  - %s  попросить у Nix больше деталей",
	NoMatchIssue: "Если ошибка распространённая, откройте issue с этим выводом:",
	HiddenOne:    "(%d кадр(ов) из nixpkgs скрыто — используйте --show-all)",
	HiddenMany:   "(%d кадр(ов) из nixpkgs скрыто — используйте --show-all)",
	Categories: map[string]string{
		"eval":     "ВЫЧИСЛЕНИЕ",
		"modules":  "МОДУЛИ",
		"packages": "ПАКЕТЫ",
		"build":    "СБОРКА",
		"store":    "ХРАНИЛИЩЕ",
		"flakes":   "ФЛЕЙКИ",
		"network":  "СЕТЬ",
		"daemon":   "ДЕМОН",
		"nixos":    "NIXOS",
	},
	Usage: usageRU,
}

const usageEN = `nix-explain — explain Nix error messages, offline.

Usage:
  nix-explain [flags] [text|file]      explain an error from a file or argument
  nix-explain < error.log              explain an error from stdin
  nix-explain run [flags] -- CMD...    run a command and explain if it fails
  nix-explain list [query]             list known errors
  nix-explain version                  print the version

Flags:
  --lang LANG     output language: en, ru, auto (default auto)
  --json          print the matched rule as JSON
  --color MODE    auto, always or never (default auto)
  --show-all      keep nixpkgs frames in the trace

Exit codes (explain): 0 matched, 2 not recognised.
Exit codes (run): the exit code of CMD.

Examples:
  nix build .#foo 2>&1 | nix-explain
  nix-explain run -- nixos-rebuild switch
  nix-explain "error: infinite recursion encountered"
`

const usageRU = `nix-explain — объясняет ошибки Nix, офлайн.

Использование:
  nix-explain [флаги] [текст|файл]     объяснить ошибку из файла или аргумента
  nix-explain < error.log              объяснить ошибку из stdin
  nix-explain run [флаги] -- КОМАНДА   запустить команду и объяснить при падении
  nix-explain list [запрос]            список известных ошибок
  nix-explain version                  версия

Флаги:
  --lang ЯЗЫК     язык вывода: en, ru, auto (по умолчанию auto)
  --json          вывести подобранное правило в JSON
  --color РЕЖИМ   auto, always, never (по умолчанию auto)
  --show-all      не скрывать кадры из nixpkgs

Коды возврата (explain): 0 — ошибка распознана, 2 — нет.
Коды возврата (run): код завершения КОМАНДЫ.

Примеры:
  nix build .#foo 2>&1 | nix-explain
  nix-explain run -- nixos-rebuild switch
  nix-explain "error: infinite recursion encountered"
`
