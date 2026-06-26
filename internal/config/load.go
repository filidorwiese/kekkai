package config

import (
	"bytes"
	"fmt"
	"os"

	kekkaiembed "github.com/filidorwiese/kekkai/embed"
	"gopkg.in/yaml.v3"
)

const ProjectConfigName = ".kekkai.yml"

// Load merges defaults → ~/.kekkai.yml → ./.kekkai.yml.
// projectDir is typically the current working directory.
func Load(home string, projectDir string) (*Config, error) {
	cfg, err := decodeDefaults()
	if err != nil {
		return nil, fmt.Errorf("decode embedded defaults: %w", err)
	}

	globalPath := home + "/.kekkai.yml"
	if err := mergeFile(cfg, globalPath); err != nil {
		return nil, err
	}

	projectPath := projectDir + "/" + ProjectConfigName
	if err := mergeFile(cfg, projectPath); err != nil {
		return nil, err
	}

	if err := Expand(cfg, home); err != nil {
		return nil, err
	}
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
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
		if l.Image.GitDeltaVersion != nil {
			cfg.Image.GitDeltaVersion = *l.Image.GitDeltaVersion
		}
		if l.Image.ZshInDockerVersion != nil {
			cfg.Image.ZshInDockerVersion = *l.Image.ZshInDockerVersion
		}
		if l.Image.TflintVersion != nil {
			cfg.Image.TflintVersion = *l.Image.TflintVersion
		}
		if l.Image.ClaudeCodeVersion != nil {
			cfg.Image.ClaudeCodeVersion = *l.Image.ClaudeCodeVersion
		}
		cfg.Image.AptPackages = append(cfg.Image.AptPackages, l.Image.AptPackages...)
	}
	cfg.Mounts = append(cfg.Mounts, l.Mounts...)
	if l.Env != nil {
		if cfg.Env == nil {
			cfg.Env = map[string]string{}
		}
		for k, v := range l.Env {
			cfg.Env[k] = v
		}
	}
	if l.Firewall != nil {
		if l.Firewall.AllowGithubMeta != nil {
			cfg.Firewall.AllowGithubMeta = *l.Firewall.AllowGithubMeta
		}
		if l.Firewall.AllowHostLan != nil {
			cfg.Firewall.AllowHostLan = *l.Firewall.AllowHostLan
		}
		cfg.Firewall.AllowedDomains = append(cfg.Firewall.AllowedDomains, l.Firewall.AllowedDomains...)
	}
	cfg.Caps = append(cfg.Caps, l.Caps...)
	if l.Claude != nil && l.Claude.Args != nil {
		cfg.Claude.Args = *l.Claude.Args
	}
}
