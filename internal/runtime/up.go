package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/filidorwiese/kekkai/internal/config"
	"github.com/filidorwiese/kekkai/internal/docker"
	"github.com/filidorwiese/kekkai/internal/firewall"
	"github.com/filidorwiese/kekkai/internal/image"
)

// UpOptions carries up subcommand flags.
type UpOptions struct {
	Force       bool
	Verbose     bool
	ExtraClaude []string // args after `--`
	Version     string
}

// Up loads config, ensures the image exists, then runs the sandbox container.
// Returns the docker exit code.
func Up(cwd string, opts UpOptions) (int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, err
	}
	cfg, err := config.Load(home, cwd)
	if err != nil {
		return 0, err
	}

	name := ContainerName(cwd)

	if existing, _ := docker.ContainerID(LabelCwd+"="+cwd, true); existing != "" {
		if !opts.Force {
			return 0, fmt.Errorf("a kekkai container for %s already exists (id %s); use --force to recreate, or `kekkai down`", cwd, existing[:12])
		}
		fmt.Fprintf(os.Stderr, "kekkai: removing existing container %s…\n", existing[:12])
		if err := docker.Quiet("rm", "-f", existing); err != nil {
			return 0, fmt.Errorf("remove existing container: %w", err)
		}
	}

	tag, err := image.EnsureImage(cfg, opts.Verbose)
	if err != nil {
		return 0, err
	}
	imgHash := strings.TrimPrefix(tag, "kekkai:")

	tmpDir, err := os.MkdirTemp("", "kekkai-"+name+"-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(tmpDir)

	fwConfPath := filepath.Join(tmpDir, "firewall.conf")
	if err := firewall.WriteConf(cfg, fwConfPath); err != nil {
		return 0, fmt.Errorf("render firewall.conf: %w", err)
	}

	args, err := buildRunArgs(cfg, cwd, name, tag, imgHash, fwConfPath, opts)
	if err != nil {
		return 0, err
	}
	return docker.Run(args...)
}

const hostDockerSocket = "/var/run/docker.sock"

// dockerAccessArgs returns the docker-run args needed to grant the sandbox
// access to the host docker daemon: a bind mount of the socket and a
// --group-add for the socket's host GID so kekkai can write to it.
func dockerAccessArgs() ([]string, error) {
	info, err := os.Stat(hostDockerSocket)
	if err != nil {
		return nil, fmt.Errorf("docker_access requires %s on the host: %w", hostDockerSocket, err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("docker_access: cannot stat %s for gid", hostDockerSocket)
	}
	gid := strconv.FormatUint(uint64(st.Gid), 10)
	return []string{
		"-v", hostDockerSocket + ":" + hostDockerSocket,
		"--group-add", gid,
	}, nil
}

func buildRunArgs(cfg *config.Config, cwd, name, tag, imgHash, fwConfPath string, opts UpOptions) ([]string, error) {
	args := []string{"run", "--rm", "-it",
		"--name", name,
		"--label", LabelCwd + "=" + cwd,
		"--label", LabelImageHash + "=" + imgHash,
		"--label", LabelVersion + "=" + opts.Version,
	}

	// NET_ADMIN + NET_RAW are required for the in-container firewall script
	// (iptables/ipset). Not configurable.
	args = append(args, "--cap-add", "NET_ADMIN", "--cap-add", "NET_RAW")

	if cfg.DockerAccess {
		extra, err := dockerAccessArgs()
		if err != nil {
			return nil, err
		}
		args = append(args, extra...)
	}

	args = append(args,
		"-v", cwd+":/workspace",
		"-v", fwConfPath+":/etc/kekkai/firewall.conf:ro",
		"-v", HistoryVolume(cwd)+":/commandhistory",
	)

	for _, m := range cfg.Mounts {
		if m.Skip {
			continue
		}
		if _, err := os.Stat(m.Source); err != nil {
			if m.Optional {
				fmt.Fprintf(os.Stderr, "kekkai: optional mount %s missing on host, skipping\n", m.Source)
				continue
			}
			fmt.Fprintf(os.Stderr, "kekkai: warning: mount source %s missing on host\n", m.Source)
		}
		spec := m.Source + ":" + m.Target
		if m.Readonly {
			spec += ":ro"
		}
		args = append(args, "-v", spec)
	}

	for _, e := range cfg.Env {
		args = append(args, "-e", e)
	}
	args = append(args, "-e", "WORKSPACE="+filepath.Base(cwd))

	claudeArgs := cfg.Claude.Args
	if len(opts.ExtraClaude) > 0 {
		claudeArgs = strings.TrimSpace(claudeArgs + " " + strings.Join(opts.ExtraClaude, " "))
	}
	args = append(args, "-e", "CLAUDE_ARGS="+claudeArgs)

	args = append(args, "-w", "/workspace")
	args = append(args, tag,
		"bash", "-c",
		"sudo /usr/local/bin/init-firewall.sh && exec claude $CLAUDE_ARGS",
	)
	return args, nil
}
