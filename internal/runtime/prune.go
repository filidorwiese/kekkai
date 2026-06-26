package runtime

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/filidorwiese/kekkai/internal/docker"
)

type PruneOptions struct {
	Volumes bool
	Yes     bool
}

// Prune removes:
//   - containers whose kekkai.cwd label points to a non-existent host folder
//   - kekkai:* images not in use by any container
//   - (with Volumes) named kekkai-history-* volumes for orphan cwds
func Prune(opts PruneOptions) error {
	orphanContainers, err := findOrphanContainers()
	if err != nil {
		return err
	}
	orphanImages, err := findOrphanImages()
	if err != nil {
		return err
	}
	var orphanVols []string
	if opts.Volumes {
		orphanVols, err = findOrphanVolumes()
		if err != nil {
			return err
		}
	}

	if len(orphanContainers) == 0 && len(orphanImages) == 0 && len(orphanVols) == 0 {
		fmt.Println("kekkai: nothing to prune")
		return nil
	}

	fmt.Println("kekkai will remove:")
	for _, c := range orphanContainers {
		fmt.Printf("  container %s (cwd=%s)\n", c.id[:12], c.cwd)
	}
	for _, ref := range orphanImages {
		fmt.Printf("  image     %s\n", ref)
	}
	for _, v := range orphanVols {
		fmt.Printf("  volume    %s\n", v)
	}

	if !opts.Yes {
		fmt.Print("proceed? [y/N] ")
		r := bufio.NewReader(os.Stdin)
		ans, _ := r.ReadString('\n')
		ans = strings.TrimSpace(strings.ToLower(ans))
		if ans != "y" && ans != "yes" {
			fmt.Println("aborted")
			return nil
		}
	}

	for _, c := range orphanContainers {
		if err := docker.Quiet("rm", "-f", c.id); err != nil {
			fmt.Fprintf(os.Stderr, "kekkai: rm container %s: %v\n", c.id[:12], err)
		}
	}
	for _, ref := range orphanImages {
		if err := docker.Quiet("rmi", ref); err != nil {
			fmt.Fprintf(os.Stderr, "kekkai: rmi image %s: %v\n", ref, err)
		}
	}
	for _, v := range orphanVols {
		if err := docker.Quiet("volume", "rm", v); err != nil {
			fmt.Fprintf(os.Stderr, "kekkai: rm volume %s: %v\n", v, err)
		}
	}
	return nil
}

type orphanContainer struct {
	id, cwd string
}

func findOrphanContainers() ([]orphanContainer, error) {
	out, err := docker.ListByLabel(LabelCwd, true, "{{.ID}}\t{{.Label \""+LabelCwd+"\"}}")
	if err != nil {
		return nil, err
	}
	var rows []orphanContainer
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		if _, err := os.Stat(parts[1]); err != nil && os.IsNotExist(err) {
			rows = append(rows, orphanContainer{id: parts[0], cwd: parts[1]})
		}
	}
	return rows, nil
}

func findOrphanImages() ([]string, error) {
	out, err := docker.ImagesByReferenceTagged("kekkai")
	if err != nil {
		return nil, err
	}
	var orphans []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		ref, id := parts[0], parts[1]
		inUse, err := docker.ImageInUse(id)
		if err != nil {
			continue
		}
		if !inUse {
			orphans = append(orphans, ref)
		}
	}
	return orphans, nil
}

func findOrphanVolumes() ([]string, error) {
	vols, err := docker.VolumesByName("kekkai-history-")
	if err != nil {
		return nil, err
	}
	containers, err := docker.ListByLabel(LabelCwd, true, "{{.Label \""+LabelCwd+"\"}}")
	if err != nil {
		return nil, err
	}
	activeHashes := map[string]bool{}
	for _, cwd := range strings.Split(strings.TrimSpace(containers), "\n") {
		if cwd == "" {
			continue
		}
		activeHashes[cwdHash(cwd)] = true
	}
	var orphans []string
	for _, v := range vols {
		hash := strings.TrimPrefix(v, "kekkai-history-")
		if !activeHashes[hash] {
			orphans = append(orphans, v)
		}
	}
	return orphans, nil
}
