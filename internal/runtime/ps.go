package runtime

import (
	"fmt"
	"strings"

	"github.com/filidorwiese/kekkai/internal/docker"
)

// Ps prints a table of all running kekkai containers.
func Ps() error {
	format := "{{.ID}}\t{{.Names}}\t{{.Status}}\t{{.Label \"" + LabelCwd + "\"}}"
	out, err := docker.ListByLabel(LabelCwd, false, format)
	if err != nil {
		return err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		fmt.Println("(no running kekkai containers)")
		return nil
	}
	fmt.Println("CONTAINER ID\tNAME\tSTATUS\tCWD")
	fmt.Println(out)
	return nil
}
