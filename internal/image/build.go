package image

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	kekkaiembed "github.com/filidorwiese/kekkai/embed"
	"github.com/filidorwiese/kekkai/internal/config"
)

// EnsureImage builds the image for cfg if `docker image inspect <tag>` misses.
// Returns the tag.
func EnsureImage(cfg *config.Config, verbose bool) (string, error) {
	tag, err := Tag(cfg)
	if err != nil {
		return "", err
	}
	if imageExists(tag) {
		return tag, nil
	}
	fmt.Fprintln(os.Stderr, "kekkai: building image (first run takes a few minutes, cached after)…")
	if err := Build(cfg, tag, verbose); err != nil {
		return "", err
	}
	return tag, nil
}

func imageExists(tag string) bool {
	cmd := exec.Command("docker", "image", "inspect", tag)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// Build renders the Dockerfile + writes init-firewall.sh into a temp build
// context, then shells out to `docker build`.
func Build(cfg *config.Config, tag string, verbose bool) error {
	df, err := RenderDockerfile(cfg)
	if err != nil {
		return err
	}

	dir, err := os.MkdirTemp("", "kekkai-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(df), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "init-firewall.sh"), kekkaiembed.InitFirewallScript, 0o755); err != nil {
		return err
	}

	args := []string{"build", "-t", tag}
	args = append(args, dir)
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	env := append(os.Environ(), "DOCKER_BUILDKIT=1")
	if verbose {
		env = append(env, "BUILDKIT_PROGRESS=plain")
	}
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build: %w", err)
	}
	return nil
}
