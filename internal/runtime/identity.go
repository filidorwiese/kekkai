package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	LabelCwd       = "kekkai.cwd"
	LabelImageHash = "kekkai.image_hash"
	LabelVersion   = "kekkai.version"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9_.-]+`)

// ContainerName builds the deterministic container name for a host directory.
func ContainerName(cwd string) string {
	base := strings.ToLower(filepath.Base(cwd))
	base = nonAlnum.ReplaceAllString(base, "_")
	if base == "" {
		base = "workspace"
	}
	return "kekkai-" + base + "-" + cwdHash(cwd)
}

// HistoryVolume is the named volume for persisted bash history, keyed by cwd.
func HistoryVolume(cwd string) string {
	return "kekkai-history-" + cwdHash(cwd)
}

// cwdHash is sha256($PWD)[:8].
func cwdHash(cwd string) string {
	sum := sha256.Sum256([]byte(cwd))
	return hex.EncodeToString(sum[:])[:8]
}
