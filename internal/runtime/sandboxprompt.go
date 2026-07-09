package runtime

import (
	"fmt"
	"strings"

	"kekkai/internal/config"
	"kekkai/internal/selfupdate"
)

// appendPromptMinVersion is the first Claude Code release where
// --append-system-prompt works in interactive mode (changelog v1.0.51);
// older versions accept it in --print mode only.
const appendPromptMinVersion = "1.0.51"

// sandboxPrompt is the pinned sandbox-awareness text (spec 011, verbatim).
// Delivered via KEKKAI_SYSTEM_PROMPT → quoted --append-system-prompt in the
// image CMD — never inline in the exec call, never a replacing flag.
const sandboxPrompt = `You are running inside Kekkai, a security sandbox (docs:
https://github.com/filidorwiese/kekkai). The environment is intentionally
restricted:

- Filesystem: only the workspace and explicitly configured mounts are visible.
  Some files may be shadowed (present but empty) because they contain secrets.
- Network: outbound traffic is limited to an allowlist. Blocked destinations
  typically fail as connection timeouts or refused connections.
- Tools: only packages installed in the sandbox image are available.

When a command fails, first consider normal causes. If the failure pattern
matches a sandbox restriction (unreachable host, missing tool, unexpectedly
empty file), do not attempt to bypass or disable the sandbox. Instead, tell the
user exactly what to add to .kekkai.yaml in the workspace root - for example a
domain under network.allowed_domains, a package under image.apt_packages, or a
mount under disk.mounts - and mention that changes take effect after restarting
with ` + "`kekkai up`" + `. Configuration reference:
https://github.com/filidorwiese/kekkai#configure

These restrictions are chosen by the user. Work within them by default.`

// supportsAppendPrompt gates injection on the resolved claude version.
// Unknown (empty) version → false: an unrecognized flag would abort claude at
// startup, the one failure mode this feature must never cause.
func supportsAppendPrompt(version string) bool {
	if version == "" {
		return false
	}
	return selfupdate.CompareVersions(version, appendPromptMinVersion) >= 0
}

// sandboxPromptFor renders the pinned prompt plus a short config summary so
// Claude can tell "blocked by sandbox" from ordinary errors without probing.
// Summary contract: ≤10 lines, ≤8 items per list then "...and N more",
// omitted entirely when there is nothing to list.
func sandboxPromptFor(cfg *config.Config) string {
	var lines []string
	if cfg.Network.AllowAll {
		lines = append(lines, "- network: unrestricted (allow_all)")
	} else {
		if l := summaryList("allowed domains", cfg.Network.AllowedDomains); l != "" {
			lines = append(lines, l)
		}
		if l := summaryList("allowed CIDRs", cfg.Network.AllowedCIDRs); l != "" {
			lines = append(lines, l)
		}
	}
	if l := summaryList("shadowed (secret) files", cfg.Secrets.Hide); l != "" {
		lines = append(lines, l)
	}
	if len(lines) == 0 {
		return sandboxPrompt
	}
	return sandboxPrompt + "\n\nCurrent sandbox config:\n" + strings.Join(lines, "\n")
}

func summaryList(label string, items []string) string {
	if len(items) == 0 {
		return ""
	}
	const max = 8
	shown := items
	if len(items) > max {
		shown = items[:max]
	}
	line := "- " + label + ": " + strings.Join(shown, ", ")
	if n := len(items) - len(shown); n > 0 {
		line += fmt.Sprintf(" ...and %d more", n)
	}
	return line
}
