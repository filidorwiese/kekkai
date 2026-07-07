//go:build darwin

package runtime

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"kekkai/internal/config"
	"kekkai/internal/docker"
)

// runtimeIdentity selects preflight fix-hints only; it never gates (§7.4).
type runtimeIdentity int

const (
	runtimeUnknown runtimeIdentity = iota
	runtimeDockerDesktop
	runtimeOrbStack
	runtimeColima
)

// Hint tables from contracts/preflight.md — message text is contract.
var bindHints = map[runtimeIdentity]string{
	runtimeDockerDesktop: "add the folder under Settings → Resources → File Sharing",
	runtimeOrbStack:      "(should not occur; report)",
	runtimeColima:        "add the path to colima's mounts (`colima start --mount <path>:w` or edit colima.yaml)",
	runtimeUnknown:       "share the folder into your runtime's VM",
}

var agentHints = map[runtimeIdentity]string{
	runtimeDockerDesktop: `enable "Allow SSH agent forwarding" / update Docker Desktop`,
	runtimeOrbStack:      "update OrbStack (forwarding is native)",
	runtimeColima:        "restart with `colima start --ssh-agent`",
	runtimeUnknown:       "expose your SSH agent at " + darwinAgentSocket + " in the VM, or set `git.ssh_agent: false`",
}

// preflight runs one throwaway container from the just-ensured image,
// read-only binding every path the real run will bind, so VM-sharing and
// agent-socket problems surface before any sandbox work (§7.4).
func preflight(cfg *config.Config, pwd, imageTag string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	args := []string{"run", "--rm",
		"-v", pwd + ":/kekkai-probe/workspace:ro",
		"-v", filepath.Join(home, ".claude") + ":/kekkai-probe/claude:ro",
	}
	if cfg.Git.Enabled {
		gitconfig := filepath.Join(home, ".gitconfig")
		if _, err := os.Stat(gitconfig); err == nil {
			args = append(args, "-v", gitconfig+":/kekkai-probe/gitconfig:ro")
		}
	}
	for i, m := range cfg.Disk.Mounts {
		if m.Skip {
			continue
		}
		if _, err := os.Stat(m.HostPath); err != nil {
			continue
		}
		args = append(args, "-v", fmt.Sprintf("%s:/kekkai-probe/m%d:ro", m.HostPath, i))
	}
	probeCmd := "true"
	if cfg.Git.SSHAgent {
		args = append(args, "-v", darwinAgentSocket+":/ssh-agent")
		probeCmd = "test -S /ssh-agent"
	}
	args = append(args, imageTag, "sh", "-c", probeCmd)

	cmd := exec.Command("docker", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err == nil {
		return nil
	}

	// docker reserves exit 125+ for daemon/run errors (bind failures land
	// there); a lower non-zero code is the probe command itself, which can
	// only be the agent-socket test.
	exitCode := -1
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	}
	if cfg.Git.SSHAgent && exitCode > 0 && exitCode < 125 {
		return preflightError("agent socket",
			darwinAgentSocket+" is missing in the runtime VM (git.ssh_agent: true)",
			agentHints[detectRuntime()])
	}
	detail := strings.TrimSpace(stderr.String())
	if detail == "" {
		detail = fmt.Sprintf("probe container failed (%v)", err)
	}
	return preflightError("bind", detail, bindHints[detectRuntime()])
}

// preflightError renders the contracts/preflight.md failure format; the
// full text is the error so main prints it without a prefix.
func preflightError(capability, detail, hint string) error {
	detail = strings.ReplaceAll(detail, "\n", "\n  ")
	return fmt.Errorf("kekkai: preflight failed — %s\n  %s\n  fix: %s", capability, detail, hint)
}

// detectRuntime identifies the docker runtime, invoked only after a probe
// failure (research.md R3): identity strings are too brittle to gate on.
func detectRuntime() runtimeIdentity {
	info, err := docker.Info()
	if err != nil {
		return runtimeUnknown
	}
	osName := strings.ToLower(info.OperatingSystem)
	name := strings.ToLower(info.Name)
	switch {
	case strings.Contains(osName, "docker desktop"):
		return runtimeDockerDesktop
	case strings.Contains(osName, "orbstack") || strings.Contains(name, "orbstack"):
		return runtimeOrbStack
	case strings.Contains(osName, "colima") || strings.Contains(name, "colima"):
		return runtimeColima
	}
	return runtimeUnknown
}
