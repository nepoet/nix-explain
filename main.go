// Command nix-explain explains Nix error messages from a local, embedded
// knowledge base. It never calls an API and works completely offline.
package main

import (
	"embed"
	"os"

	"github.com/nepoet/nix-explain/internal/cli"
	"github.com/nepoet/nix-explain/internal/kb"
)

//go:embed rules
var rulesFS embed.FS

// version is overridden at build time:
//
//	go build -ldflags "-X main.version=1.2.3"
var version = "dev"

func main() {
	rules, err := kb.Load(rulesFS, "rules")
	if err != nil {
		os.Stderr.WriteString("nix-explain: cannot load rules: " + err.Error() + "\n")
		os.Exit(1)
	}
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, rules, version))
}
