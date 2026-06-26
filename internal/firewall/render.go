package firewall

import (
	"strings"

	"github.com/filidorwiese/kekkai/internal/config"
)

// EnvVars renders the firewall settings as KEY=value strings to inject with
// `docker run -e`. Consumed by embed/init-firewall.sh, which reads ALLOW_HOST_LAN
// and ALLOWED_DOMAINS from its environment.
//
// Passing these via env (rather than a bind-mounted file) keeps the firewall a
// runtime input while avoiding host-path bind mounts, which are unreliable across
// snap-confined, SELinux, rootless, and remote docker daemons.
func EnvVars(cfg *config.Config) []string {
	return []string{
		"ALLOW_HOST_LAN=" + boolToFlag(cfg.Firewall.AllowHostLan),
		"ALLOWED_DOMAINS=" + strings.Join(cfg.Firewall.AllowedDomains, " "),
	}
}

func boolToFlag(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
