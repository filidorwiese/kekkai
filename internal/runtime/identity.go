// Package runtime implements the kekkai subcommands.
//
// identity.go is the single source of container/volume/image identity (§7.1):
// every consumer (up/down/shell/exec/traffic/mpr/ps/prune) derives names,
// labels, the mirrored project path and the mirrored Claude config dir from
// here.
package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"kekkai/internal/config"
)

// ProjectDir is the sole source of the project path (specs/026): the
// kekkai.cwd label, container/volume names, every bind source and
// destination, and the run/exec working directory all derive from it.
// Symlinks are resolved because Claude Code keys per-project state by the
// kernel cwd (the real path), and mirroring that path is the whole point —
// os.Getwd alone would return $PWD with symlinks intact.
func ProjectDir() (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(pwd)
	if err != nil {
		return "", err
	}
	return filepath.Clean(real), nil
}

// ClaudeConfigDir is the sole source of the host Claude config dir
// (specs/028): $CLAUDE_CONFIG_DIR when set and non-empty, else ~/.claude.
// Unlike ProjectDir, symlinks are NOT resolved: Claude Code records absolute
// paths (plugin installs, marketplaces, hook commands) rooted at its own
// unresolved notion of the config dir, and the mount destination must equal
// that literal prefix for those paths to resolve inside the sandbox
// (research R1). A symlinked home would otherwise dangle every stored path.
func ClaudeConfigDir() (string, error) {
	if v := os.Getenv("CLAUDE_CONFIG_DIR"); v != "" {
		return filepath.Abs(filepath.Clean(v))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

// protectedContainerPaths are kekkai's own in-container locations. A project
// bound at or above one of them would hide the sandbox user's home (bashrc,
// nvm, the ~/.claude symlink), the claude/firewall/mpr binaries, or the
// history volume (research R4). Descendants are fine: a nested mount hides
// nothing.
var protectedContainerPaths = []string{"/home/kekkai", "/usr/local/bin", "/commandhistory"}

// ValidateProjectPath reports every reason the project path p cannot be
// mirrored into the sandbox (specs/026 contracts/sandbox-layout.md).
func ValidateProjectPath(p string) []error {
	return validateMirrorPath("project path", p, "move or rename the project")
}

// ValidateClaudeConfigDir applies the same rules to the Claude config dir
// (specs/028 contracts/sandbox-layout.md).
func ValidateClaudeConfigDir(p string) []error {
	return validateMirrorPath("claude config dir", p, "move it or set CLAUDE_CONFIG_DIR")
}

// validateMirrorPath reports every reason p cannot be mirrored at its own
// path inside the sandbox; kind labels the messages and remedy closes the
// unmountable-char one. Pure string work, no docker: it joins the one-pass
// §4.4 report in `up`. Ancestor checks are component-wise so /home/kekkai2
// is not mistaken for a parent of /home/kekkai. ':' is unrepresentable in
// `-v src:dst`; control characters would break the line-oriented `docker ps`
// output ContainersByLabel parses.
func validateMirrorPath(kind, p, remedy string) []error {
	var errs []error
	if p == "/" {
		errs = append(errs, fmt.Errorf("%s / cannot be mirrored into the sandbox (root directory)", kind))
	} else {
		for _, protected := range protectedContainerPaths {
			if isAncestorOrSelf(p, protected) {
				errs = append(errs, fmt.Errorf("%s %s cannot be mirrored into the sandbox: it would overlay %s", kind, p, protected))
			}
		}
	}
	if hasUnmountableChar(p) {
		errs = append(errs, fmt.Errorf("%s %s contains ':' or control characters, which the container runtime cannot express as a mount destination; %s", kind, p, remedy))
	}
	return errs
}

// isAncestorOrSelf reports whether child equals dir or lies beneath it,
// component-wise (so /a/bc is not under /a/b).
func isAncestorOrSelf(dir, child string) bool {
	rel, err := filepath.Rel(dir, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
}

// hasUnmountableChar: ':' (the -v separator) or any C0 control byte.
func hasUnmountableChar(p string) bool {
	for i := 0; i < len(p); i++ {
		if p[i] == ':' || p[i] < 0x20 {
			return true
		}
	}
	return false
}

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

// ImageTag returns kekkai:<sha256(rendered Dockerfile + init-firewall.sh +
// kekkai-mpr.py)[:12]>. Only bake-time inputs enter the hash (§6.1).
func ImageTag(renderedDockerfile, firewallScript, mprScript string) string {
	return "kekkai:" + shortHash(renderedDockerfile+firewallScript+mprScript, 12)
}

// ConfigHash is the version-independent bake-input hash stored as the
// kekkai.config_hash image label. It keys the §6.2 offline fallback only,
// never builds. Inputs: the platform constants (Debian base, nvm tag), the
// node_version selector, apt packages, firewall + mpr scripts, sandbox
// uid/gid, apt repos — the bake inputs minus the claude version. Identity is included
// so the fallback never reuses an image baked for a different host user
// (specs/018); repos so it never reuses one baked for different repos
// (specs/020). An empty repo list serializes to "" — hashes from before the
// apt_repos feature stay valid.
func ConfigHash(nodeVersion string, aptPackages []string, aptRepos []config.AptRepo, firewallScript, mprScript string, uid, gid int) string {
	var repos strings.Builder
	for _, r := range aptRepos {
		repos.WriteString("\n" + r.Name + "|" + r.URL + "|" + r.Suite + "|" + r.Components + "|" + r.KeyURL)
	}
	return shortHash(config.DebianBaseImage+"\n"+config.NvmVersion+"\n"+nodeVersion+
		"\n"+strings.Join(aptPackages, " ")+"\n"+firewallScript+"\n"+mprScript+
		"\n"+strconv.Itoa(uid)+":"+strconv.Itoa(gid)+repos.String(), 12)
}
