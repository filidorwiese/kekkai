package runtime

import (
	"fmt"
	"os"
	"path/filepath"
)

// starterConfig: every line is a comment or blank, so a fresh init parses as
// an empty document and runs on pure defaults (§4.5). Commented example
// values equal the code defaults (copy/paste safety); uncomment to change.
const starterConfig = `# .kekkai.yaml - kekkai sandbox configuration
# This file is optional: kekkai runs on defaults without it.
# Every setting below is optional - uncomment to change it.

# image:
#   # Node.js version for the sandbox: "lts" (default), "current", or a version like "24"
#   node_version: lts
#
#   # Extra apt packages baked into the image, appended to kekkai's builtin set.
#   apt_packages: [golang]

# claude:
#   # "latest" (default) resolves the newest release at 'kekkai up', so a new
#   # Claude release triggers an image rebuild. Pin an exact version ("2.0.14")
#   # for a stable image.
#   version: latest
#
#   # Passed to claude verbatim, REPLACING the default - keep the flag below if
#   # you want autonomous mode. Example with a model: "--dangerously-skip-permissions --model opus"
#   args: "--dangerously-skip-permissions"

# git:
#   # true: mounts ~/.gitconfig (readonly) - your identity and settings;
#   # the agent can create local commits.
#   # false (or section omitted): .git is mounted readonly - the agent
#   # can read history (log, diff, show) but not commit or rewrite it.
#   enabled: true
#
#   # Exposes your SSH agent ($SSH_AUTH_SOCK) and allowed_signers file:
#   # enables SSH commit signing and push/pull to allowed hosts.
#   # Off by default - the agent can then act with all your loaded keys.
#   # Requires git.enabled: true.
#   ssh_agent: false

# disk:
#   mounts:
#     - source: ~/.aws              # host path
#       target: /home/kekkai/.aws   # optional - inferred when omitted
#       readonly: true
#       optional: true              # skip silently when the source is missing

# env:
#   # Extra environment for the sandbox (map, not list). ${VAR} passes host
#   # values through.
#   NODE_ENV: development
#   GH_TOKEN: ${GH_TOKEN}

# network:
#   # Omitted network section = egress firewall on
#   # Note: api.anthropic.com (required by Claude Code) is always allowed.
#
#   # GitHub git/api/ssh via the api.github.com/meta CIDR list:
#   allow_github: true
#
#   # Extra domains, resolved to IPs once at sandbox start:
#   allowed_domains:
#     - registry.npmjs.org
#
#   # Literal IP ranges, e.g. your LAN or a staging network:
#   allowed_cidrs:
#     - 192.168.1.0/24
#
#   # Escape hatch: true disables the egress firewall entirely.
#   # Must be the ONLY network key when set.
#   allow_all: false

# secrets:
#   # Exact file/dir paths relative to the workspace root (no globs) that the
#   # agent must not read. Files read empty; directories become empty tmpfs.
#   hide:
#     - .env.production
#     - deploy/certs

# limits:
#   # Container resource caps; unlimited when omitted.
#   cpus: 4
#   memory: 8g
`

// Init writes the starter .kekkai.yaml; errors if a config already exists.
func Init() error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	// Typo check first: writing a fresh .kekkai.yaml next to a .kekkai.yml
	// would leave two config-looking files with only one ever read (specs/012).
	if _, err := os.Lstat(filepath.Join(pwd, ".kekkai.yml")); err == nil {
		return fmt.Errorf("found .kekkai.yml - kekkai only reads .kekkai.yaml; rename it before running 'kekkai init'")
	}
	if _, err := os.Stat(filepath.Join(pwd, ".kekkai.yaml")); err == nil {
		return fmt.Errorf(".kekkai.yaml already exists, not overwriting")
	}
	path := filepath.Join(pwd, ".kekkai.yaml")
	if err := os.WriteFile(path, []byte(starterConfig), 0o644); err != nil {
		return err
	}
	fmt.Println("wrote .kekkai.yaml - review it, then run 'kekkai up'")
	return nil
}
