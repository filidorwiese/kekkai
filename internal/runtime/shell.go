package runtime

import (
	"fmt"

	"github.com/filidorwiese/kekkai/internal/docker"
)

func Shell(cwd string) (int, error) {
	id, err := docker.ContainerID(LabelCwd+"="+cwd, false)
	if err != nil {
		return 0, err
	}
	if id == "" {
		return 0, fmt.Errorf("no running kekkai container for %s — run `kekkai up` first", cwd)
	}
	return docker.Run("exec", "-it", id, "zsh")
}
