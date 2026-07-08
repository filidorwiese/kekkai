// Package config loads and validates .kekkai.yaml (§4).
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Defaults are code constants — no layered or user-global config (§4.1).
const (
	DefaultNodeVersion   = "lts"
	DefaultClaudeVersion = "latest"
	DefaultClaudeArgs    = "--dangerously-skip-permissions"

	// debianRelease pins the Debian flavor of the node base image; the user
	// only picks the node version (§4.2).
	debianRelease = "trixie"
)

// ErrNoConfig is Discover's not-found signal. Since the config file became
// optional (§4.1), `up` answers it with a warning plus Defaults(), not an
// abort; other callers may still require the file.
var ErrNoConfig = fmt.Errorf("no .kekkai.yaml found, run 'kekkai init'")

type Config struct {
	Image   ImageConfig       `yaml:"image"`
	Claude  ClaudeConfig      `yaml:"claude"`
	Git     GitConfig         `yaml:"git"`
	Disk    DiskConfig        `yaml:"disk"`
	Env     map[string]string `yaml:"env"`
	Network NetworkConfig     `yaml:"network"`
	Secrets SecretsConfig     `yaml:"secrets"`
	Limits  LimitsConfig      `yaml:"limits"`

	// networkKeysSet records which network keys appear in the document, so
	// allow_all exclusivity catches explicit `allow_github: false` too.
	// limitsKeysSet distinguishes `cpus: 0` from an absent key.
	// imageKeysSet distinguishes `node_version: ""` from an absent key.
	networkKeysSet []string
	limitsKeysSet  []string
	imageKeysSet   []string
}

type ImageConfig struct {
	NodeVersion string   `yaml:"node_version"`
	AptPackages []string `yaml:"apt_packages"`
}

// ResolvedBaseImage is the only place the node version becomes an image
// reference; the Debian release is pinned, never user input (§4.2).
func (i ImageConfig) ResolvedBaseImage() string {
	return "node:" + i.NodeVersion + "-" + debianRelease
}

type ClaudeConfig struct {
	Version string `yaml:"version"`
	Args    string `yaml:"args"`
}

type GitConfig struct {
	Enabled  bool `yaml:"enabled"`
	SSHAgent bool `yaml:"ssh_agent"`
}

type DiskConfig struct {
	Mounts []Mount `yaml:"mounts"`
}

type Mount struct {
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	ReadOnly bool   `yaml:"readonly"`
	Optional bool   `yaml:"optional"`

	// Resolved by Validate (§4.3): expanded host path and inferred target.
	HostPath      string `yaml:"-"`
	ContainerPath string `yaml:"-"`
	// Skip marks optional mounts whose ${VAR} source is unset (§4.3).
	Skip bool `yaml:"-"`
}

type NetworkConfig struct {
	AllowAll       bool     `yaml:"allow_all"`
	AllowGithub    bool     `yaml:"allow_github"`
	AllowedCIDRs   []string `yaml:"allowed_cidrs"`
	AllowedDomains []string `yaml:"allowed_domains"`
}

type SecretsConfig struct {
	Hide []string `yaml:"hide"`
}

type LimitsConfig struct {
	CPUs   float64 `yaml:"cpus"`
	Memory string  `yaml:"memory"`
}

// legacyKeys are pre-rewrite schema keys that get a targeted migration error
// instead of a bare unknown-key message (§4.1).
var legacyKeys = map[string]string{
	"image.base":                "image.node_version",
	"image.base_image":          "image.node_version",
	"image.claude_code_version": "claude.version",
	"firewall":                  "network",
	"docker_access":             "(removed — docker-in-sandbox is not supported)",
	"mounts":                    "disk.mounts",
}

// Discover finds the config file in dir. Both .kekkai.yml and .kekkai.yaml
// present is an error; neither present returns ErrNoConfig.
func Discover(dir string) (string, error) {
	ymlPath := filepath.Join(dir, ".kekkai.yml")
	yamlPath := filepath.Join(dir, ".kekkai.yaml")
	ymlExists := fileExists(ymlPath)
	yamlExists := fileExists(yamlPath)
	switch {
	case ymlExists && yamlExists:
		return "", fmt.Errorf("both .kekkai.yml and .kekkai.yaml exist, remove one")
	case ymlExists:
		return ymlPath, nil
	case yamlExists:
		return yamlPath, nil
	default:
		return "", ErrNoConfig
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// Load discovers, parses (strict) and applies defaults. Parse-level
// violations (unknown keys, legacy keys, type errors) are collected, not
// fail-fast, so Validate can extend the same one-pass report (§4.4).
// A nil Config is returned only when nothing could be read at all.
func Load(dir string) (*Config, []error) {
	path, err := Discover(dir)
	if err != nil {
		return nil, []error{err}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []error{fmt.Errorf("read %s: %w", path, err)}
	}

	var errs []error
	errs = append(errs, detectLegacyKeys(data)...)

	cfg := &Config{}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	// io.EOF = empty document (zero-byte, whitespace- or comments-only file):
	// presence is the opt-in, emptiness means defaults (§4.1).
	if err := dec.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
		if typeErr, ok := err.(*yaml.TypeError); ok {
			// yaml.v3 fills what it can and accumulates field errors.
			for _, msg := range typeErr.Errors {
				errs = append(errs, fmt.Errorf("%s: %s", filepath.Base(path), msg))
			}
		} else {
			return nil, append(errs, fmt.Errorf("parse %s: %w", path, err))
		}
	}

	cfg.networkKeysSet = presentSectionKeys(data, "network")
	cfg.limitsKeysSet = presentSectionKeys(data, "limits")
	cfg.imageKeysSet = presentSectionKeys(data, "image")
	cfg.applyDefaults()
	return cfg, errs
}

// Defaults is the configuration of a project without a config file: the zero
// Config with all defaults applied (§4.1).
func Defaults() *Config {
	cfg := &Config{}
	cfg.applyDefaults()
	return cfg
}

func (c *Config) applyDefaults() {
	if c.Image.NodeVersion == "" && !c.keySet(c.imageKeysSet, "node_version") {
		c.Image.NodeVersion = DefaultNodeVersion
	}
	if c.Claude.Version == "" {
		c.Claude.Version = DefaultClaudeVersion
	}
	if c.Claude.Args == "" {
		c.Claude.Args = DefaultClaudeArgs
	}
}

// detectLegacyKeys pre-scans the raw document for pre-rewrite schema keys.
func detectLegacyKeys(data []byte) []error {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil // strict decode reports the real problem
	}
	var errs []error
	present := func(key string) bool {
		parts := strings.SplitN(key, ".", 2)
		if len(parts) == 1 {
			_, ok := raw[key]
			return ok
		}
		section, ok := raw[parts[0]].(map[string]any)
		if !ok {
			return false
		}
		_, ok = section[parts[1]]
		return ok
	}
	for key, replacement := range legacyKeys {
		if present(key) {
			errs = append(errs, fmt.Errorf(
				"config schema changed: %q is now %s — run 'kekkai init' and see the README",
				key, replacement))
		}
	}
	return errs
}

// keySet reports whether key appears explicitly in the document section.
func (c *Config) keySet(keys []string, key string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}

func presentSectionKeys(data []byte, section string) []string {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil
	}
	m, ok := raw[section].(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

var varPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandVars expands ${VAR} references and reports unset variables.
func expandVars(s string) (string, []string) {
	var missing []string
	out := varPattern.ReplaceAllStringFunc(s, func(m string) string {
		name := varPattern.FindStringSubmatch(m)[1]
		val, ok := os.LookupEnv(name)
		if !ok {
			missing = append(missing, name)
			return m
		}
		return val
	})
	return out, missing
}

// expandTilde expands a leading ~ to the host home directory.
func expandTilde(s, home string) string {
	if s == "~" {
		return home
	}
	if strings.HasPrefix(s, "~/") {
		return filepath.Join(home, s[2:])
	}
	return s
}
