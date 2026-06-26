package runtime

import (
	"fmt"
	"net"
	"os"
	"os/exec"

	"github.com/filidorwiese/kekkai/internal/config"
	"github.com/filidorwiese/kekkai/internal/docker"
)

// Doctor runs host-environment checks and prints a colored pass/warn/fail list.
// Binary, daemon, and config checks are blocking; the rest are warnings.
// Returns non-zero if any blocking check fails.
func Doctor(cwd string) (int, error) {
	colorOn = wantColor()

	failures := 0
	warnings := 0

	for _, bin := range []string{"docker", "git", "curl"} {
		if _, err := exec.LookPath(bin); err != nil {
			fail("%s not found in PATH", bin)
			failures++
		} else {
			pass("%s in PATH", bin)
		}
	}

	if err := docker.Quiet("info"); err != nil {
		fail("docker daemon unreachable (group membership? socket permissions?)")
		failures++
	} else {
		pass("docker daemon reachable")
	}

	home, err := os.UserHomeDir()
	cfg, cfgErr := (*config.Config)(nil), error(nil)
	if err != nil {
		fail("cannot find home dir: %v", err)
		failures++
	} else if cfg, cfgErr = config.Load(home, cwd); cfgErr != nil {
		fail("config invalid: %v", cfgErr)
		failures++
	} else {
		pass("config parses and validates")
	}

	if cfg != nil {
		for _, m := range cfg.Mounts {
			if m.Skip {
				warn("mount %s skipped (optional, env var unset)", m.Target)
				warnings++
			} else if _, err := os.Stat(m.Source); err != nil {
				warn("mount source missing: %s", m.Source)
				warnings++
			}
		}
		for _, d := range cfg.Firewall.AllowedDomains {
			if _, err := net.LookupHost(d); err != nil {
				warn("allowed domain does not resolve: %s", d)
				warnings++
			}
		}
	}

	if sock := os.Getenv("SSH_AUTH_SOCK"); sock == "" {
		warn("SSH_AUTH_SOCK unset (git push from the sandbox will fail)")
		warnings++
	} else if _, err := os.Stat(sock); err != nil {
		warn("SSH_AUTH_SOCK socket unreachable: %s", sock)
		warnings++
	}

	summary(failures, warnings)
	if failures > 0 {
		return 1, nil
	}
	return 0, nil
}

const (
	cReset  = "\033[0m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cRed    = "\033[31m"
	cBold   = "\033[1m"
)

var colorOn bool

// wantColor enables ANSI output only when stderr is a terminal and the user
// has not opted out via NO_COLOR / TERM=dumb.
func wantColor() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	fi, err := os.Stderr.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func paint(code, s string) string {
	if !colorOn {
		return s
	}
	return code + s + cReset
}

func pass(f string, a ...any) { item(cGreen, "✓", f, a...) }
func warn(f string, a ...any) { item(cYellow, "!", f, a...) }
func fail(f string, a ...any) { item(cRed, "✗", f, a...) }

func item(code, sym, f string, a ...any) {
	fmt.Fprintln(os.Stderr, "  "+paint(code, sym+" "+fmt.Sprintf(f, a...)))
}

func summary(failures, warnings int) {
	fmt.Fprintln(os.Stderr)
	switch {
	case failures > 0:
		fmt.Fprintln(os.Stderr, paint(cRed+cBold, fmt.Sprintf("✗ %d failed, %d warnings", failures, warnings)))
	case warnings > 0:
		fmt.Fprintln(os.Stderr, paint(cYellow+cBold, fmt.Sprintf("! all checks passed, %d warnings", warnings)))
	default:
		fmt.Fprintln(os.Stderr, paint(cGreen+cBold, "✓ all checks passed"))
	}
}
