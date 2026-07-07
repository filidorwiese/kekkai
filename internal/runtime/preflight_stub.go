//go:build !darwin

package runtime

import "kekkai/internal/config"

// preflight is darwin-only (§7.4); zero overhead elsewhere.
func preflight(cfg *config.Config, pwd, imageTag string) error {
	return nil
}
