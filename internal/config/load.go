package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	kekkaiembed "github.com/filidorwiese/kekkai/embed"
	"gopkg.in/yaml.v3"
)

// ConfigBaseName is the config filename without extension; both .yml and .yaml are accepted.
const ConfigBaseName = ".kekkai"

// Load merges embedded defaults → ./.kekkai.{yml,yaml}.
// projectDir is typically the current working directory; home is used to expand
// ~ in the merged result.
func Load(home string, projectDir string) (*Config, error) {
	cfg, err := decodeDefaults()
	if err != nil {
		return nil, fmt.Errorf("decode embedded defaults: %w", err)
	}

	projectPath, err := resolveConfigPath(projectDir)
	if err != nil {
		return nil, err
	}
	if err := mergeFile(cfg, projectPath); err != nil {
		return nil, err
	}

	cfg.Env = dedupEnv(cfg.Env)

	if err := Expand(cfg, home); err != nil {
		return nil, err
	}
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Source returns the project config file path that Load merges on top of the
// embedded defaults. An empty string means no file was found. It surfaces the
// same ambiguity error as Load (both extensions present).
func Source(projectDir string) (project string, err error) {
	return resolveConfigPath(projectDir)
}

// resolveConfigPath probes dir for .kekkai.yml and .kekkai.yaml. It returns the
// existing one, "" if neither exists, or an error if both exist (ambiguous).
func resolveConfigPath(dir string) (string, error) {
	ymlPath := dir + "/" + ConfigBaseName + ".yml"
	yamlPath := dir + "/" + ConfigBaseName + ".yaml"
	ymlOK := fileExists(ymlPath)
	yamlOK := fileExists(yamlPath)
	switch {
	case ymlOK && yamlOK:
		return "", fmt.Errorf("both %s and %s exist; keep only one", ymlPath, yamlPath)
	case ymlOK:
		return ymlPath, nil
	case yamlOK:
		return yamlPath, nil
	default:
		return "", nil
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// dedupEnv keeps only the last occurrence of each KEY in a list of KEY=value
// entries, so later layers override earlier ones per key. Order follows each
// key's final position.
func dedupEnv(entries []string) []string {
	last := map[string]int{}
	for i, e := range entries {
		last[envKey(e)] = i
	}
	out := make([]string, 0, len(entries))
	for i, e := range entries {
		if last[envKey(e)] == i {
			out = append(out, e)
		}
	}
	return out
}

func envKey(e string) string {
	if i := strings.IndexByte(e, '='); i >= 0 {
		return e[:i]
	}
	return e
}

func decodeDefaults() (*Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(kekkaiembed.DefaultsYAML))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func mergeFile(cfg *Config, path string) error {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var l layer
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&l); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	mergeLayer(cfg, &l)
	return nil
}

func mergeLayer(cfg *Config, l *layer) {
	if l.Image != nil {
		if l.Image.Base != nil {
			cfg.Image.Base = *l.Image.Base
		}
		if l.Image.ClaudeCodeVersion != nil {
			cfg.Image.ClaudeCodeVersion = *l.Image.ClaudeCodeVersion
		}
		if l.Image.DockerCliVersion != nil {
			cfg.Image.DockerCliVersion = *l.Image.DockerCliVersion
		}
		cfg.Image.AptPackages = append(cfg.Image.AptPackages, l.Image.AptPackages...)
	}
	cfg.Mounts = append(cfg.Mounts, l.Mounts...)
	cfg.Env = append(cfg.Env, l.Env...)
	if l.Firewall != nil {
		if l.Firewall.AllowHostLan != nil {
			cfg.Firewall.AllowHostLan = *l.Firewall.AllowHostLan
		}
		cfg.Firewall.AllowedDomains = append(cfg.Firewall.AllowedDomains, l.Firewall.AllowedDomains...)
	}
	if l.Claude != nil && l.Claude.Args != nil {
		cfg.Claude.Args = *l.Claude.Args
	}
	if l.DockerAccess != nil {
		cfg.DockerAccess = *l.DockerAccess
	}
}
