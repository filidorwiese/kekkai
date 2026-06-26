package docker

import (
	"strings"
)

// ContainerID returns the first container ID matching the given label k=v, or
// empty string if none is found. `all` includes stopped containers.
func ContainerID(label string, all bool) (string, error) {
	args := []string{"ps", "--filter", "label=" + label, "--format", "{{.ID}}"}
	if all {
		args = []string{"ps", "-a", "--filter", "label=" + label, "--format", "{{.ID}}"}
	}
	out, err := Output(args...)
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", nil
	}
	// take the first line if multiple
	return strings.SplitN(out, "\n", 2)[0], nil
}

// ListByLabel returns parsed rows for `docker ps -a --filter label=<label>`
// with the requested format columns.
func ListByLabel(label string, all bool, format string) (string, error) {
	args := []string{"ps", "--filter", "label=" + label, "--format", format}
	if all {
		args = []string{"ps", "-a", "--filter", "label=" + label, "--format", format}
	}
	return Output(args...)
}

// ImageIDs returns the IDs of all images carrying the given label.
func ImageIDs(label string) ([]string, error) {
	out, err := Output("images", "--filter", "label="+label, "--format", "{{.ID}}", "--no-trunc")
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

// ImagesByReference returns IDs of images whose repository matches ref
// (e.g. "kekkai").
func ImagesByReference(ref string) ([]string, error) {
	out, err := Output("images", "--filter", "reference="+ref, "--format", "{{.ID}}", "--no-trunc")
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

// ImagesByReferenceTagged returns "repo:tag IMAGE_ID" rows.
func ImagesByReferenceTagged(ref string) (string, error) {
	return Output("images", "--filter", "reference="+ref, "--format", "{{.Repository}}:{{.Tag}}\t{{.ID}}")
}

// VolumesByName returns volume names matching prefix.
func VolumesByName(prefix string) ([]string, error) {
	out, err := Output("volume", "ls", "--format", "{{.Name}}")
	if err != nil {
		return nil, err
	}
	var matching []string
	for _, name := range splitLines(out) {
		if strings.HasPrefix(name, prefix) {
			matching = append(matching, name)
		}
	}
	return matching, nil
}

// ContainerLabel returns the value of label key on the given container.
func ContainerLabel(id, key string) (string, error) {
	out, err := Output("inspect", "--format", "{{ index .Config.Labels \""+key+"\" }}", id)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ImageInUse reports whether any container (running or stopped) is using the
// given image ID.
func ImageInUse(imageID string) (bool, error) {
	out, err := Output("ps", "-a", "--filter", "ancestor="+imageID, "--format", "{{.ID}}")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func splitLines(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
