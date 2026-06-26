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

	reserved := map[string]string{
		"WORKSPACE":       "injected automatically",
		"ALLOW_HOST_LAN":  "set via firewall.allow_host_lan",
		"ALLOWED_DOMAINS": "set via firewall.allowed_domains",
	}
	for i, e := range cfg.Env {
		eq := strings.IndexByte(e, '=')
		if eq <= 0 {
			return fmt.Errorf("env[%d] (%q): must be KEY=value", i, e)
		}
		if why, ok := reserved[e[:eq]]; ok {
			return fmt.Errorf("env[%d]: %s is %s and cannot be set here", i, e[:eq], why)
		}
	}

	for _, d := range cfg.Firewall.AllowedDomains {
		if strings.ContainsAny(d, " \t\n") {
			return fmt.Errorf("firewall.allowed_domains: %q contains whitespace", d)
		}
	}
	return nil
}
