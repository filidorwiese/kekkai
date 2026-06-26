package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var envVarRE = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandStr replaces ~ prefix with home and ${VAR} occurrences with the env
// value. If a referenced variable is unset, ok=false and the original string
// is returned alongside the offending var name.
func expandStr(s, home string) (out string, missingVar string, ok bool) {
	if strings.HasPrefix(s, "~/") {
		s = home + s[1:]
	} else if s == "~" {
		s = home
	}
	out = envVarRE.ReplaceAllStringFunc(s, func(m string) string {
		name := m[2 : len(m)-1]
		v, set := os.LookupEnv(name)
		if !set || v == "" {
			if missingVar == "" {
				missingVar = name
			}
			return m
		}
		return v
	})
	return out, missingVar, missingVar == ""
}

// Expand resolves ~ and ${VAR} across all string fields in cfg.
// Mounts whose source resolves to a missing ${VAR} are marked Skip=true if
// optional; otherwise it is an error.
func Expand(cfg *Config, home string) error {
	for i := range cfg.Mounts {
		src, miss, ok := expandStr(cfg.Mounts[i].Source, home)
		if !ok {
			if cfg.Mounts[i].Optional {
				cfg.Mounts[i].Skip = true
				continue
			}
			return fmt.Errorf("mount %s: env var %s is unset (mark mount optional to skip)", cfg.Mounts[i].Target, miss)
		}
		cfg.Mounts[i].Source = src

		tgt, miss, ok := expandStr(cfg.Mounts[i].Target, home)
		if !ok {
			return fmt.Errorf("mount target: env var %s is unset", miss)
		}
		cfg.Mounts[i].Target = tgt
	}

	for k, v := range cfg.Env {
		expanded, miss, ok := expandStr(v, home)
		if !ok {
			return fmt.Errorf("env %s: env var %s is unset", k, miss)
		}
		cfg.Env[k] = expanded
	}

	if cfg.Claude.Args != "" {
		expanded, miss, ok := expandStr(cfg.Claude.Args, home)
		if !ok {
			return fmt.Errorf("claude.args: env var %s is unset", miss)
		}
		cfg.Claude.Args = expanded
	}
	return nil
}
