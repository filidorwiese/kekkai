package runtime

import (
	"fmt"
	"os"

	"golang.org/x/term"

	"kekkai/internal/docker"
)

// Exec runs a command in the running sandbox for $PWD, resolved by label,
// and passes its exit code through. A TTY is allocated only when stdin is a
// terminal so piped invocations keep working (research.md R1).
func Exec(cmdArgs []string) (int, error) {
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
			args := []string{"exec", "-i"}
			if term.IsTerminal(int(os.Stdin.Fd())) {
				args = append(args, "-t")
			}
			args = append(args, c.ID)
			return docker.Interactive(append(args, cmdArgs...)...)
		}
	}
	return 1, fmt.Errorf("no running sandbox for %s, run 'kekkai up'", pwd)
}
