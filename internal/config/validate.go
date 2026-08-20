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
	// nodeVersionPattern: exactly the four accepted forms — lts, major,
	// major.minor, full (§4.2). Installer aliases (node, stable, lts/*,
	// lts/<codename>) are deliberately rejected: the value set is kekkai's
	// contract, not nvm's.
	nodeVersionPattern = regexp.MustCompile(`^(lts|[0-9]+(\.[0-9]+){0,2})$`)

	// apt_repos grammars (specs/020 contracts/config-validation.md). Strict
	// allowlists: every field lands inside the Dockerfile's RUN line, so
	// whitespace, quotes, `$`, `[`, `]`, `=` and all other shell/apt
	// metacharacters must be unrepresentable — apt options like trusted=yes
	// are excluded structurally, not by pattern-matching for them.
	aptRepoNamePattern   = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)
	aptRepoURLPattern    = regexp.MustCompile(`^https://[A-Za-z0-9._~%/+:-]+$`)
	aptSuitePattern      = regexp.MustCompile(`^[A-Za-z0-9._+-]+$`)
	aptFlatSuitePattern  = regexp.MustCompile(`^(\./|([A-Za-z0-9._+-]+/)+)$`)
	aptComponentsPattern = regexp.MustCompile(`^[a-z0-9-]+$`)
)

// aptRepoReservedNames collide with the builtin GitHub CLI repo files.
var aptRepoReservedNames = []string{"github-cli", "githubcli"}

// Validate runs every semantic check from contracts/config.md and resolves
// mount expansion/target inference (§4.3). All violations are collected so
// `up` reports them in one pass before any docker work (§4.4).
func Validate(cfg *Config) []error {
	var errs []error
	fail := func(format string, a ...any) {
		errs = append(errs, fmt.Errorf(format, a...))
	}

	// image.node_version: plain version selector; absent key already
	// defaulted to lts, explicit empty is a mistake, not a default request
	if strings.TrimSpace(cfg.Image.NodeVersion) == "" {
		fail("image.node_version must not be empty (omit the key for the default %q)", DefaultNodeVersion)
	} else if !nodeVersionPattern.MatchString(cfg.Image.NodeVersion) {
		fail("image.node_version must be \"lts\", a major (\"22\"), major.minor (\"22.11\"), or full version (\"22.11.0\"), got %q", cfg.Image.NodeVersion)
	}

	// image.apt_repos: allowlist grammar per field, duplicate + reserved
	// names, flat-repo rules (specs/020 contracts/config-validation.md)
	repoNames := map[string]int{}
	for i, r := range cfg.Image.AptRepos {
		label := fmt.Sprintf("image.apt_repos[%d]", i)
		if aptRepoNamePattern.MatchString(r.Name) {
			label = fmt.Sprintf("image.apt_repos[%d] (%s)", i, r.Name)
		}

		switch {
		case r.Name == "":
			fail("%s: name is required", label)
		case !aptRepoNamePattern.MatchString(r.Name):
			fail("%s: name %q must be 1-64 chars of [a-z0-9-]", label, r.Name)
		case r.Name == aptRepoReservedNames[0] || r.Name == aptRepoReservedNames[1]:
			fail("%s: name %q is reserved for the builtin GitHub CLI repository", label, r.Name)
		default:
			if prev, dup := repoNames[r.Name]; dup {
				fail("%s: duplicate name %q (already used by image.apt_repos[%d])", label, r.Name, prev)
			} else {
				repoNames[r.Name] = i
			}
		}

		switch {
		case r.URL == "":
			fail("%s: url is required", label)
		case !strings.HasPrefix(r.URL, "https://"):
			fail("%s: url must start with https://, got %q", label, r.URL)
		case !aptRepoURLPattern.MatchString(r.URL):
			fail("%s: url %q contains characters outside [A-Za-z0-9._~%%/+:-]", label, r.URL)
		}

		if r.KeyURL != "" {
			switch {
			case !strings.HasPrefix(r.KeyURL, "https://"):
				fail("%s: key_url must start with https://, got %q", label, r.KeyURL)
			case !aptRepoURLPattern.MatchString(r.KeyURL):
				fail("%s: key_url %q contains characters outside [A-Za-z0-9._~%%/+:-]", label, r.KeyURL)
			}
		}

		flat := strings.HasSuffix(r.Suite, "/")
		switch {
		case r.Suite == "":
			fail("%s: suite is required", label)
		case flat && (strings.Contains(r.Suite, "..") || !aptFlatSuitePattern.MatchString(r.Suite)):
			fail("%s: flat suite %q must be \"./\" or slash-terminated segments of [A-Za-z0-9._+-] without \"..\"", label, r.Suite)
		case !flat && !aptSuitePattern.MatchString(r.Suite):
			fail("%s: suite %q must be chars of [A-Za-z0-9._+-] (end with \"/\" for a flat repository)", label, r.Suite)
		}

		if r.Components != "" {
			if flat {
				fail("%s: components must be omitted for flat repositories (suite ends with \"/\")", label)
			} else if !aptComponentsPattern.MatchString(r.Components) {
				fail("%s: components %q must be a single component of [a-z0-9-]", label, r.Components)
			}
		}
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
