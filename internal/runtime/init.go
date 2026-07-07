package runtime

import (
	"fmt"
	"os"
	"path/filepath"
)

// starterConfig: active values equal the code defaults (copy/paste safety,
// §4.5); every optional section is present but commented, with README-grade
// comments; behavior-changing examples appear only in comments.
const starterConfig = `# .kekkai.yaml - kekkai sandbox configuration
# The container is the security boundary: Claude runs fully autonomous inside it.
# Sections that are commented out are disabled.

image:
  # Node.js version for the sandbox: "lts" (default), "current", or a version like "24"
  node_version: lts

  # Extra apt packages baked into the image, appended to kekkai's builtin set.
  # apt_packages: [golang]

claude:
  # "latest" (default) resolves the newest release at 'kekkai up', so a new
  # Claude release triggers an image rebuild. Pin an exact version ("2.0.14")
  # for a stable image.
  version: latest

  # Passed to claude verbatim, REPLACING the default - keep the flag below if
  # you want autonomous mode. Example with a model: "--dangerously-skip-permissions --model opus"
  args: "--dangerously-skip-permissions"

# git:
#   # true: mounts your ~/.gitconfig read-only so commits carry your identity.
#   # false/omitted: .git is bound read-only - history readable, commits fail.
#   enabled: true
#
#   # true: mounts $SSH_AUTH_SOCK so git push/pull authenticates as you.
#   # Requires enabled: true. The agent can then use your keys against any
#   # allowed host - combine with a tight network section.
#   ssh_agent: false

# disk:
#   mounts:
#     - source: ~/.aws              # host path; ~ and ${VAR} expand
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
#   # Note: api.anthropic.com and statsig.anthropic.com are always allowed.
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
#   # Escape hatch: true disables the egress firewall entirely. Must be the
#   # ONLY network key when set.
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
	for _, name := range []string{".kekkai.yml", ".kekkai.yaml"} {
		if _, err := os.Stat(filepath.Join(pwd, name)); err == nil {
			return fmt.Errorf("%s already exists, not overwriting", name)
		}
	}
	path := filepath.Join(pwd, ".kekkai.yaml")
	if err := os.WriteFile(path, []byte(starterConfig), 0o644); err != nil {
		return err
	}
	fmt.Println("wrote .kekkai.yaml - review it, then run 'kekkai up'")
	return nil
}
