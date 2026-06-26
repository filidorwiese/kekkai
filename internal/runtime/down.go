package runtime

import (
	"fmt"
	"os"

	"github.com/filidorwiese/kekkai/internal/docker"
)

func Down(cwd string) error {
	id, err := docker.ContainerID(LabelCwd+"="+cwd, true)
	if err != nil {
		return err
	}
	if id == "" {
		fmt.Fprintf(os.Stderr, "kekkai: no container found for %s\n", cwd)
		return nil
	}
	if err := docker.Quiet("rm", "-f", id); err != nil {
		return fmt.Errorf("docker rm: %w", err)
	}
	fmt.Fprintf(os.Stderr, "kekkai: removed container %s\n", id[:12])
	return nil
}
