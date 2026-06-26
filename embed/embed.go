// Package embed holds assets baked into the kekkai binary at build time:
// the embedded YAML defaults, the Dockerfile template, and the firewall script.
package embed

import _ "embed"

//go:embed defaults.yml
var DefaultsYAML []byte

//go:embed Dockerfile.tmpl
var DockerfileTemplate string

//go:embed init-firewall.sh
var InitFirewallScript []byte
