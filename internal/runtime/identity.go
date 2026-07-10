// Package runtime implements the kekkai subcommands.
//
// identity.go is the single source of container/volume/image identity (§7.1):
// every consumer (up/down/shell/ps/prune) derives names and labels from here.
package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"kekkai/internal/config"
)

// sandboxIdentity returns the uid/gid baked into the sandbox user (specs/018):
// the host identity when both ids are in the user range (>= 1000), else the
// historical 1000/1000. The gate keeps root and system-range ids out of the
// image — mapping a system gid (e.g. 20 dialout, 100 users) would enroll the
// sandbox user in that group's privileges — and makes darwin (501/20) render
// the same Dockerfile as before without a GOOS branch. Never configurable:
// identity is a bake input (§6.1), not runtime config.
func sandboxIdentity() (uid, gid int) {
	uid, gid = os.Getuid(), os.Getgid()
	if uid < 1000 || gid < 1000 {
		return 1000, 1000
	}
	return uid, gid
}

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
// never builds. Inputs: the platform constants (Debian base, nvm tag), the
// node_version selector, apt packages, firewall script, sandbox uid/gid —
// the bake inputs minus the claude version. Identity is included so the
// fallback never reuses an image baked for a different host user (specs/018).
func ConfigHash(nodeVersion string, aptPackages []string, firewallScript string, uid, gid int) string {
	return shortHash(config.DebianBaseImage+"\n"+config.NvmVersion+"\n"+nodeVersion+
		"\n"+strings.Join(aptPackages, " ")+"\n"+firewallScript+
		"\n"+strconv.Itoa(uid)+":"+strconv.Itoa(gid), 12)
}
