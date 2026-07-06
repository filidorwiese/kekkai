package runtime

import (
	"fmt"
	"os"

	"kekkai/internal/docker"
)

// Down stops and removes the sandbox container for $PWD, resolved by the
// kekkai.cwd label only. Nothing found is not an error (contracts/cli.md).
func Down() error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	containers, err := docker.ContainersByLabel(LabelCwd + "=" + pwd)
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		fmt.Printf("no sandbox found for %s\n", pwd)
		return nil
	}
	for _, c := range containers {
		if err := docker.RemoveContainer(c.ID); err != nil {
			return err
		}
		fmt.Printf("removed %s\n", c.Name)
	}
	return nil
}
