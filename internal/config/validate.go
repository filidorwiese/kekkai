package config

import (
	"fmt"
	"strings"
)

// Validate runs semantic checks on the merged config.
func Validate(cfg *Config) error {
	if strings.TrimSpace(cfg.Image.Base) == "" {
		return fmt.Errorf("image.base is required")
	}

	seen := map[string]int{}
	for i, m := range cfg.Mounts {
		if m.Skip {
			continue
		}
		if m.Target == "" {
			return fmt.Errorf("mounts[%d]: target is required", i)
		}
		if m.Source == "" {
			return fmt.Errorf("mounts[%d] (%s): source is required", i, m.Target)
		}
		if prev, ok := seen[m.Target]; ok {
			return fmt.Errorf("mounts[%d] and mounts[%d] both target %s", prev, i, m.Target)
		}
		seen[m.Target] = i
	}

	for i, e := range cfg.Env {
		eq := strings.IndexByte(e, '=')
		if eq <= 0 {
			return fmt.Errorf("env[%d] (%q): must be KEY=value", i, e)
		}
		if e[:eq] == "WORKSPACE" {
			return fmt.Errorf("env[%d]: WORKSPACE is injected automatically and cannot be set", i)
		}
	}

	for _, d := range cfg.Firewall.AllowedDomains {
		if strings.ContainsAny(d, " \t\n") {
			return fmt.Errorf("firewall.allowed_domains: %q contains whitespace", d)
		}
	}
	return nil
}
