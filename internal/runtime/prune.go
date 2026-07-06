package runtime

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"kekkai/internal/docker"
)

// Prune removes orphan kekkai containers and unused kekkai:* images;
// --volumes adds history volumes. Running sandboxes' resources are never
// touched. Confirmation is required unless --yes.
func Prune(includeVolumes, yes bool) error {
	containers, err := docker.ContainersByLabel(LabelCwd)
	if err != nil {
		return err
	}

	var orphans []docker.Container
	inUseImageHash := map[string]bool{}
	inUseVolume := map[string]bool{}
	for _, c := range containers {
		if c.Running {
			inUseImageHash[c.ImageHash] = true
			inUseVolume[HistoryVolume(c.Cwd)] = true
		} else {
			orphans = append(orphans, c)
		}
	}

	images, err := docker.KekkaiImages()
	if err != nil {
		return err
	}
	var unusedImages []string
	for _, img := range images {
		hash := strings.TrimPrefix(img.Tag, "kekkai:")
		if !inUseImageHash[hash] {
			unusedImages = append(unusedImages, img.Tag)
		}
	}

	var unusedVolumes []string
	if includeVolumes {
		volumes, err := docker.Volumes("kekkai-history-")
		if err != nil {
			return err
		}
		for _, v := range volumes {
			if !inUseVolume[v] {
				unusedVolumes = append(unusedVolumes, v)
			}
		}
	}

	if len(orphans)+len(unusedImages)+len(unusedVolumes) == 0 {
		fmt.Println("nothing to prune")
		return nil
	}

	fmt.Println("will remove:")
	for _, c := range orphans {
		fmt.Printf("  container %s (%s)\n", c.Name, c.Cwd)
	}
	for _, tag := range unusedImages {
		fmt.Printf("  image %s\n", tag)
	}
	for _, v := range unusedVolumes {
		fmt.Printf("  volume %s\n", v)
	}

	if !yes {
		fmt.Print("proceed? [y/N] ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		answer := strings.ToLower(strings.TrimSpace(line))
		if answer != "y" && answer != "yes" {
			fmt.Println("aborted")
			return nil
		}
	}

	for _, c := range orphans {
		if err := docker.RemoveContainer(c.ID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
	}
	for _, tag := range unusedImages {
		if err := docker.RemoveImage(tag); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
	}
	for _, v := range unusedVolumes {
		if err := docker.RemoveVolume(v); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
	}
	fmt.Println("prune complete")
	return nil
}
