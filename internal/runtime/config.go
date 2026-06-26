package runtime

import (
	"fmt"
	"os"
	"strings"

	"github.com/filidorwiese/kekkai/internal/config"
)

// Config loads and validates the merged configuration for cwd, then prints a
// human-readable summary. A non-nil error means the configuration is invalid.
func Config(cwd string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Source surfaces the same ambiguity error Load would; resolve first so we
	// can report which file contributes even when the merge later fails.
	project, srcErr := config.Source(cwd)

	cfg, err := config.Load(home, cwd)
	if err != nil {
		if srcErr == nil {
			printSources(project)
		}
		return fmt.Errorf("invalid configuration: %w", err)
	}

	printSources(project)
	printConfig(cfg)
	fmt.Println("Configuration is valid.")
	return nil
}

func printSources(project string) {
	fmt.Println("Sources (merged in order):")
	fmt.Printf("  defaults  %s\n", "embedded")
	fmt.Printf("  project   %s\n", orNone(project))
	fmt.Println()
}

func printConfig(cfg *config.Config) {
	var b strings.Builder
	p := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }

	p("Image:\n")
	p("  base                 %s\n", cfg.Image.Base)
	p("  claude_code_version  %s\n", orDash(cfg.Image.ClaudeCodeVersion))
	p("  docker_cli_version   %s\n", orDash(cfg.Image.DockerCliVersion))
	p("  apt_packages         %s\n", orDash(strings.Join(cfg.Image.AptPackages, ", ")))
	p("\n")

	p("Mounts:\n")
	p("  $PWD -> /workspace  (writable, always)\n")
	for _, m := range cfg.Mounts {
		var flags []string
		if m.Readonly {
			flags = append(flags, "ro")
		}
		if m.Optional {
			flags = append(flags, "optional")
		}
		if m.Skip {
			flags = append(flags, "skipped: source var unset")
		}
		suffix := ""
		if len(flags) > 0 {
			suffix = "  (" + strings.Join(flags, ", ") + ")"
		}
		p("  %s -> %s%s\n", m.Source, m.Target, suffix)
	}
	p("\n")

	p("Environment:\n")
	if len(cfg.Env) == 0 {
		p("  (none)\n")
	}
	for _, e := range cfg.Env {
		p("  %s\n", e)
	}
	p("  WORKSPACE=<$PWD basename>  (injected)\n")
	p("\n")

	p("Firewall:\n")
	p("  allow_host_lan   %t\n", cfg.Firewall.AllowHostLan)
	p("  allowed_domains  (%d):\n", len(cfg.Firewall.AllowedDomains))
	for _, d := range cfg.Firewall.AllowedDomains {
		p("    %s\n", d)
	}
	p("\n")

	p("Claude args: %s\n", orDash(cfg.Claude.Args))
	p("\n")

	p("Docker access: %t\n", cfg.DockerAccess)
	if cfg.DockerAccess {
		p("  WARNING: egress via the docker socket bypasses the kekkai firewall.\n")
	}
	p("\n")

	fmt.Print(b.String())
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
