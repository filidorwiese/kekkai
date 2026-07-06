// Package runtime implements the kekkai subcommands.
//
// identity.go is the single source of container/volume/image identity (§7.1):
// every consumer (up/down/shell/ps/prune) derives names and labels from here.
package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
)

const (
	// LabelCwd is the authoritative container key: resolution is by label,
	// never by name.
	LabelCwd        = "kekkai.cwd"
	LabelImageHash  = "kekkai.image_hash"
	LabelVersion    = "kekkai.version"
	LabelConfigHash = "kekkai.config_hash"
)

func shortHash(input string, n int) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])[:n]
}

// sanitizeName lowercases and maps every char outside [a-z0-9_.-] to '-'.
// The "kekkai-" prefix guarantees a valid leading char for docker.
func sanitizeName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '.', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}

// ContainerName returns kekkai-<sanitized-basename>-<sha256(pwd)[:8]>.
func ContainerName(pwd string) string {
	return "kekkai-" + sanitizeName(filepath.Base(pwd)) + "-" + shortHash(pwd, 8)
}

// HistoryVolume returns kekkai-history-<sha256(pwd)[:8]>.
func HistoryVolume(pwd string) string {
	return "kekkai-history-" + shortHash(pwd, 8)
}

// ImageTag returns kekkai:<sha256(rendered Dockerfile + init-firewall.sh)[:12]>.
// Only bake-time inputs enter the hash (§6.1).
func ImageTag(renderedDockerfile, firewallScript string) string {
	return "kekkai:" + shortHash(renderedDockerfile+firewallScript, 12)
}

// ConfigHash is the version-independent bake-input hash stored as the
// kekkai.config_hash image label. It keys the §6.2 offline fallback only,
// never builds.
func ConfigHash(baseImage string, aptPackages []string, firewallScript string) string {
	return shortHash(baseImage+"\n"+strings.Join(aptPackages, " ")+"\n"+firewallScript, 12)
}
