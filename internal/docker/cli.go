// Package docker shells out to the docker CLI (research.md R1): daemon
// discovery, TTY handling and context resolution come free with the binary.
package docker

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// run executes docker with args and returns trimmed stdout.
func run(args ...string) (string, error) {
	cmd := exec.Command("docker", args...)
	var out, errBuf strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("docker %s: %s", args[0], msg)
	}
	return strings.TrimSpace(out.String()), nil
}

// ServerInfo identifies the docker server for preflight hint selection
// (§7.4); identity decorates error messages only, never gates.
type ServerInfo struct {
	OperatingSystem string
	Name            string
}

// Info returns the server's OperatingSystem and Name from `docker info`.
func Info() (ServerInfo, error) {
	out, err := run("info", "--format", "{{.OperatingSystem}}|{{.Name}}")
	if err != nil {
		return ServerInfo{}, err
	}
	os, name, _ := strings.Cut(out, "|")
	return ServerInfo{OperatingSystem: os, Name: name}, nil
}

// ImageExists reports whether the tag resolves via `docker image inspect`.
func ImageExists(tag string) bool {
	cmd := exec.Command("docker", "image", "inspect", tag)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// BuildImage builds contextDir (containing Dockerfile) as tag.
// verbose switches buildkit to plain progress.
func BuildImage(tag, contextDir string, labels map[string]string, verbose bool) error {
	args := []string{"build", "-t", tag}
	for k, v := range labels {
		args = append(args, "--label", k+"="+v)
	}
	if verbose {
		args = append(args, "--progress=plain")
	}
	args = append(args, contextDir)
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}
	return nil
}

// Container is one `docker ps` row scoped to kekkai labels.
type Container struct {
	ID        string
	Name      string
	Status    string
	Running   bool
	Cwd       string
	ImageHash string
	Version   string
}

const psFormat = "{{.ID}}\t{{.Names}}\t{{.Status}}\t{{.State}}\t" +
	`{{.Label "kekkai.cwd"}}` + "\t" + `{{.Label "kekkai.image_hash"}}` + "\t" + `{{.Label "kekkai.version"}}`

// ContainersByLabel lists all containers (running or not) matching filter,
// e.g. "kekkai.cwd" or "kekkai.cwd=/path".
func ContainersByLabel(filter string) ([]Container, error) {
	out, err := run("ps", "-a", "--filter", "label="+filter, "--format", psFormat)
	if err != nil {
		return nil, err
	}
	var containers []Container
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 7 {
			continue
		}
		containers = append(containers, Container{
			ID: f[0], Name: f[1], Status: f[2],
			Running: f[3] == "running",
			Cwd:     f[4], ImageHash: f[5], Version: f[6],
		})
	}
	return containers, nil
}

// RemoveContainer force-removes (stop + rm) by ID.
func RemoveContainer(id string) error {
	_, err := run("rm", "-f", id)
	return err
}

// Image is one kekkai:* image.
type Image struct {
	Tag        string
	ID         string
	Created    time.Time
	ConfigHash string
}

// KekkaiImages lists kekkai:* images, newest first.
func KekkaiImages() ([]Image, error) {
	out, err := run("images", "kekkai", "--format", "{{.Repository}}:{{.Tag}}\t{{.ID}}")
	if err != nil {
		return nil, err
	}
	var images []Image
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 2 || strings.HasSuffix(f[0], ":<none>") {
			continue
		}
		img := Image{Tag: f[0], ID: f[1]}
		meta, err := run("image", "inspect", img.Tag, "--format",
			"{{.Created}}\t"+`{{index .Config.Labels "kekkai.config_hash"}}`)
		if err == nil {
			mf := strings.Split(meta, "\t")
			if len(mf) == 2 {
				img.Created, _ = time.Parse(time.RFC3339Nano, mf[0])
				img.ConfigHash = mf[1]
			}
		}
		images = append(images, img)
	}
	sort.Slice(images, func(i, j int) bool { return images[i].Created.After(images[j].Created) })
	return images, nil
}

// RemoveImage removes an image by tag.
func RemoveImage(tag string) error {
	_, err := run("rmi", tag)
	return err
}

// Volumes lists volume names matching the name filter.
func Volumes(nameFilter string) ([]string, error) {
	out, err := run("volume", "ls", "--filter", "name="+nameFilter, "--format", "{{.Name}}")
	if err != nil {
		return nil, err
	}
	var vols []string
	for _, v := range strings.Split(out, "\n") {
		if v != "" {
			vols = append(vols, v)
		}
	}
	return vols, nil
}

// RemoveVolume removes a volume by name.
func RemoveVolume(name string) error {
	_, err := run("volume", "rm", name)
	return err
}
