package config

import (
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Reserved env keys are managed by kekkai itself (§4.3).
var reservedEnvKeys = []string{
	"WORKSPACE", "ALLOW_ALL", "ALLOW_GITHUB",
	"ALLOWED_DOMAINS", "ALLOWED_CIDRS", "SSH_AUTH_SOCK",
}

var (
	versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$`)
	memoryPattern  = regexp.MustCompile(`(?i)^[0-9]+(\.[0-9]+)?[bkmg]?$`)
)

// Validate runs every semantic check from contracts/config.md and resolves
// mount expansion/target inference (§4.3). All violations are collected so
// `up` reports them in one pass before any docker work (§4.4).
func Validate(cfg *Config) []error {
	var errs []error
	fail := func(format string, a ...any) {
		errs = append(errs, fmt.Errorf(format, a...))
	}

	// image.base_image required, node:* only
	if cfg.Image.BaseImage == "" {
		fail("image.base_image is required (e.g. %s)", DefaultBaseImage)
	} else if !strings.HasPrefix(cfg.Image.BaseImage, "node:") {
		fail("image.base_image must be a node:* image, got %q", cfg.Image.BaseImage)
	}

	// claude.version: latest or exact npm version
	if cfg.Claude.Version != "latest" && !versionPattern.MatchString(cfg.Claude.Version) {
		fail("claude.version must be \"latest\" or an exact npm version, got %q", cfg.Claude.Version)
	}

	// mounts: source required, expansion, target inference, duplicate targets
	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		home = ""
	}
	targets := map[string]int{}
	for i := range cfg.Disk.Mounts {
		m := &cfg.Disk.Mounts[i]
		label := fmt.Sprintf("disk.mounts[%d]", i)
		if m.Source == "" {
			fail("%s: source is required", label)
			continue
		}
		expanded, missing := expandVars(m.Source)
		if len(missing) > 0 {
			if m.Optional {
				m.Skip = true
				continue
			}
			fail("%s: source %q references unset variable(s) %s (set the variable or mark the mount optional)",
				label, m.Source, strings.Join(missing, ", "))
			continue
		}
		m.HostPath = expandTilde(expanded, home)

		switch {
		case m.Target != "":
			m.ContainerPath = m.Target
		case strings.HasPrefix(m.Source, "~"):
			m.ContainerPath = expandTilde(m.Source, "/home/kekkai")
		case filepath.IsAbs(m.HostPath):
			m.ContainerPath = m.HostPath
		default:
			fail("%s: cannot infer target for relative source %q, set target explicitly", label, m.Source)
			continue
		}
		if !filepath.IsAbs(m.ContainerPath) {
			fail("%s: target %q must be an absolute path", label, m.ContainerPath)
			continue
		}
		if prev, dup := targets[m.ContainerPath]; dup {
			fail("%s: duplicate target %q (already used by disk.mounts[%d])", label, m.ContainerPath, prev)
		}
		targets[m.ContainerPath] = i
	}

	// env: reserved keys, ${VAR} expansion
	for _, key := range reservedEnvKeys {
		if _, set := cfg.Env[key]; set {
			fail("env.%s is reserved and managed by kekkai", key)
		}
	}
	for key, val := range cfg.Env {
		expanded, missing := expandVars(val)
		if len(missing) > 0 {
			fail("env.%s references unset variable(s) %s", key, strings.Join(missing, ", "))
			continue
		}
		cfg.Env[key] = expandTilde(expanded, home)
	}

	// git: ssh_agent requires enabled
	if cfg.Git.SSHAgent && !cfg.Git.Enabled {
		fail("git.ssh_agent: true requires git.enabled: true")
	}

	// network: allow_all is exclusive of every other network key
	if cfg.Network.AllowAll {
		for _, key := range cfg.networkKeysSet {
			if key != "allow_all" {
				fail("network.allow_all: true cannot be combined with network.%s (the escape hatch must be deliberate and alone)", key)
			}
		}
	}
	for i, cidr := range cfg.Network.AllowedCIDRs {
		if _, err := netip.ParsePrefix(cidr); err != nil {
			fail("network.allowed_cidrs[%d]: %q is not a valid CIDR", i, cidr)
		}
	}
	for i, domain := range cfg.Network.AllowedDomains {
		if strings.ContainsAny(domain, " \t\n") || domain == "" {
			fail("network.allowed_domains[%d]: %q must be a single domain without whitespace", i, domain)
		}
	}

	// limits
	if cfg.Limits.CPUs < 0 || (cfg.Limits.CPUs == 0 && limitsCPUsSet(cfg)) {
		fail("limits.cpus must be a positive number, got %v", cfg.Limits.CPUs)
	}
	if cfg.Limits.Memory != "" && !memoryPattern.MatchString(cfg.Limits.Memory) {
		fail("limits.memory must match docker --memory grammar (e.g. 8g), got %q", cfg.Limits.Memory)
	}

	return errs
}

// limitsCPUsSet reports whether cpus was explicitly present; a zero value
// with the key absent is simply "unlimited".
func limitsCPUsSet(cfg *Config) bool {
	for _, k := range cfg.limitsKeysSet {
		if k == "cpus" {
			return true
		}
	}
	return false
}
