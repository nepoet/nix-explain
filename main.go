// Command nix-explain explains Nix error messages from a local, embedded
// knowledge base. It never calls an API and works completely offline.
package main

import (
	"embed"
	"os"
	"runtime/debug"

	"github.com/nepoet/nix-explain/internal/cli"
	"github.com/nepoet/nix-explain/internal/kb"
)

//go:embed rules i18n
var assets embed.FS

// version is overridden at build time:
//
//	go build -ldflags "-X main.version=1.2.3"
var version = "dev"

func main() {
	if version == "dev" {
		// When installed with `go install module@version` there is no
		// ldflags override, but the build info still knows the version.
		if bi, ok := debug.ReadBuildInfo(); ok {
			if v := bi.Main.Version; v != "" && v != "(devel)" {
				version = v
			}
		}
	}

	rules, err := kb.Load(assets, "rules")
	if err != nil {
		os.Stderr.WriteString("nix-explain: cannot load rules: " + err.Error() + "\n")
		os.Exit(1)
	}
	translations, err := kb.LoadTranslations(assets, "i18n", "ru")
	if err != nil {
		os.Stderr.WriteString("nix-explain: cannot load translations: " + err.Error() + "\n")
		os.Exit(1)
	}

	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, rules, translations, version))
}
