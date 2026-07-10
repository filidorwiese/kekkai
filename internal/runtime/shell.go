package runtime

import (
	"fmt"
	"os"

	"kekkai/internal/docker"
)

// Shell opens bash in the running sandbox for $PWD, resolved by label.
func Shell() (int, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return 1, err
	}
	containers, err := docker.ContainersByLabel(LabelCwd + "=" + pwd)
	if err != nil {
		return 1, err
	}
	for _, c := range containers {
		if c.Running {
			return docker.Interactive("exec", "-it", c.ID, "bash")
		}
	}
	return 1, fmt.Errorf("no running sandbox for %s, run 'kekkai up'", pwd)
}
