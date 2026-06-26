package image

import (
	"crypto/sha256"
	"encoding/hex"

	kekkaiembed "github.com/filidorwiese/kekkai/embed"
	"github.com/filidorwiese/kekkai/internal/config"
)

// Hash returns sha256 over the rendered Dockerfile + the (static) firewall
// script content. Truncated to 12 hex chars for the docker tag.
func Hash(cfg *config.Config) (string, error) {
	df, err := RenderDockerfile(cfg)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write([]byte(df))
	h.Write(kekkaiembed.InitFirewallScript)
	return hex.EncodeToString(h.Sum(nil))[:12], nil
}

// Tag returns the full kekkai:<hash> docker tag.
func Tag(cfg *config.Config) (string, error) {
	h, err := Hash(cfg)
	if err != nil {
		return "", err
	}
	return "kekkai:" + h, nil
}

// HashOfString — helper exposed for tests / inspection.
func HashOfString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
