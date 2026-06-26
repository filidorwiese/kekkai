package runtime

import (
	"fmt"
	"net"
	"os"
	"os/exec"

	"github.com/filidorwiese/kekkai/internal/config"
	"github.com/filidorwiese/kekkai/internal/docker"
)

// Doctor runs three tiers of host-environment checks.
// Tier 1+2 are blocking; Tier 3 are warnings. Returns non-zero if any blocking
// check fails.
func Doctor(cwd string) (int, error) {
	failures := 0
	warnings := 0

	section("Tier 1: required binaries & docker daemon")
	for _, bin := range []string{"docker", "git", "curl"} {
		if _, err := exec.LookPath(bin); err != nil {
			fail("%s not in PATH", bin)
			failures++
		} else {
			pass("%s in PATH", bin)
		}
	}
	if err := docker.Quiet("info"); err != nil {
		fail("docker daemon not reachable (group membership? socket permissions?)")
		failures++
	} else {
		pass("docker daemon reachable")
	}

	section("Tier 2: config parses and validates")
	home, err := os.UserHomeDir()
	if err != nil {
		fail("user home dir: %v", err)
		failures++
	} else {
		if _, err := config.Load(home, cwd); err != nil {
			fail("config: %v", err)
			failures++
		} else {
			pass("merged config (defaults + ~/.kekkai.yml + ./.kekkai.yml) parses and validates")
		}
	}

	section("Tier 3: host environment (warnings only)")
	cfg, cfgErr := func() (*config.Config, error) {
		if home == "" {
			return nil, fmt.Errorf("no home dir")
		}
		return config.Load(home, cwd)
	}()
	if cfgErr == nil && cfg != nil {
		for _, m := range cfg.Mounts {
			if m.Skip {
				warn("mount %s skipped (env var unset, optional)", m.Target)
				warnings++
				continue
			}
			if _, err := os.Stat(m.Source); err != nil {
				warn("mount source missing on host: %s (target %s)", m.Source, m.Target)
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
		warn("SSH_AUTH_SOCK is unset (git push from inside the sandbox will fail)")
		warnings++
	} else if _, err := os.Stat(sock); err != nil {
		warn("SSH_AUTH_SOCK=%s exists in env but socket is unreachable", sock)
		warnings++
	}

	fmt.Fprintln(os.Stderr)
	if failures > 0 {
		fmt.Fprintf(os.Stderr, "doctor: %d blocking failure(s), %d warning(s)\n", failures, warnings)
		return 1, nil
	}
	fmt.Fprintf(os.Stderr, "doctor: ok (%d warning(s))\n", warnings)
	return 0, nil
}

func section(s string) { fmt.Fprintf(os.Stderr, "\n[ %s ]\n", s) }
func pass(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "  ok    "+f+"\n", a...)
}
func warn(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "  warn  "+f+"\n", a...)
}
func fail(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "  FAIL  "+f+"\n", a...)
}
