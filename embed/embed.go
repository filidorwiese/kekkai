// Package assets embeds the sandbox build inputs into the binary (§1, R8):
// the distributed artifact stays a single static file.
package assets

import _ "embed"

//go:embed Dockerfile.tmpl
var DockerfileTmpl string

//go:embed init-firewall.sh
var FirewallScript string
