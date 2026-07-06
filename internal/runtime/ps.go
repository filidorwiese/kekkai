package runtime

import (
	"fmt"
	"os"
	"text/tabwriter"

	"kekkai/internal/docker"
)

// Ps lists all containers carrying the kekkai.cwd label.
func Ps() error {
	containers, err := docker.ContainersByLabel(LabelCwd)
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		fmt.Println("no kekkai sandboxes")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tCWD\tIMAGE\tSTATUS")
	for _, c := range containers {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.Name, c.Cwd, c.ImageHash, c.Status)
	}
	return w.Flush()
}
