package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	assets "kekkai/embed"
	"kekkai/internal/config"
	"kekkai/internal/docker"
	"kekkai/internal/selfupdate"
)

// Builtin apt packages (§5.1) — code constants, user apt_packages appends.
// jq/aggregate stay baked even though only the allow_github path uses them:
// the image must be identical regardless of runtime config.
var builtinAptPackages = []string{
	// firewall/lifecycle
	"sudo", "iptables", "ipset", "iproute2", "dnsutils",
	"curl", "ca-certificates", "jq", "aggregate",
	// subcommands
	"zsh",
	// convenience
	"git", "gh", "less", "nano", "procps",
}

const npmLatestURL = "https://registry.npmjs.org/@anthropic-ai/claude-code/latest"

// darwinAgentSocket is where macOS runtimes (Docker Desktop, OrbStack,
// colima --ssh-agent) forward the host SSH agent inside their VM (§5.2).
const darwinAgentSocket = "/run/host-services/ssh-auth.sock"

type UpOptions struct {
	Force   bool
	Verbose bool
	// ExtraClaudeArgs are the args after -- , appended to claude.args.
	ExtraClaudeArgs []string
	// Version is the kekkai binary version, stored as the kekkai.version label.
	Version string
}

// Up validates first (aborting before any docker work), resolves the claude
// version, builds the image on hash miss, assembles run args and hands the
// terminal to `docker run --rm -it` (§6, §7).
func Up(opts UpOptions) (int, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return 1, err
	}

	cfg, errs := config.Load(pwd)
	if cfg == nil && len(errs) == 1 && errors.Is(errs[0], config.ErrNoConfig) {
		warnNoConfig()
		cfg, errs = config.Defaults(), nil
	}
	if cfg != nil {
		errs = append(errs, config.Validate(cfg)...)
		// At `up`, ssh_agent without a host socket is a hard error (§4.4).
		// Darwin uses the runtime VM socket instead; preflight verifies it (§7.4).
		if cfg.Git.SSHAgent && goruntime.GOOS != "darwin" && os.Getenv("SSH_AUTH_SOCK") == "" {
			errs = append(errs, fmt.Errorf("git.ssh_agent is true but $SSH_AUTH_SOCK is not set on the host"))
		}
	}
	if len(errs) == 1 && cfg == nil {
		return 1, errs[0]
	}
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "invalid configuration (%d violation(s)):\n", len(errs))
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %v\n", e)
		}
		return 1, nil
	}

	// Update check runs concurrently with image/container work and is
	// read non-blockingly at the handoff: never awaited, never an error,
	// silent on the error paths above (§3).
	noticeCh := make(chan string, 1)
	go func() { noticeCh <- selfupdate.Notice(opts.Version) }()

	// Refuse a second sandbox for the same directory unless --force (§7.2).
	existing, err := docker.ContainersByLabel(LabelCwd + "=" + pwd)
	if err != nil {
		return 1, err
	}
	if len(existing) > 0 {
		if !opts.Force {
			return 1, fmt.Errorf("sandbox %s already exists for this directory — use 'kekkai up --force' to recreate, or 'kekkai down'",
				existing[0].Name)
		}
		for _, c := range existing {
			fmt.Printf("removing existing sandbox %s\n", c.Name)
			if err := docker.RemoveContainer(c.ID); err != nil {
				return 1, err
			}
		}
	}

	imageTag, claudeVersion, err := ensureImage(cfg, opts.Verbose)
	if err != nil {
		return 1, err
	}

	// darwin capability probe (§7.4); no-op elsewhere.
	if err := preflight(cfg, pwd, imageTag); err != nil {
		return 1, err
	}

	args, err := buildRunArgs(cfg, pwd, imageTag, claudeVersion, opts)
	if err != nil {
		return 1, err
	}

	select {
	case msg := <-noticeCh:
		if msg != "" {
			fmt.Println(yellow(os.Stdout, msg))
		}
	default:
		// Check not finished — silent this run, goroutine abandoned.
	}
	return docker.Interactive(args...)
}

// yellow wraps msg in the advisory yellow only when f is a terminal and
// NO_COLOR is unset (https://no-color.org). Both advisory lines (missing
// config, update notice) go through here so the convention cannot diverge.
func yellow(f *os.File, msg string) string {
	if info, err := f.Stat(); err == nil &&
		info.Mode()&os.ModeCharDevice != 0 && os.Getenv("NO_COLOR") == "" {
		return "\033[33m" + msg + "\033[0m"
	}
	return msg
}

// warnNoConfig prints the missing-config advisory (contract): one stderr line.
func warnNoConfig() {
	fmt.Fprintln(os.Stderr, yellow(os.Stderr,
		"warning: no .kekkai.yaml found, using defaults - run 'kekkai init' to customize"))
}

// ensureImage resolves the claude version, renders the Dockerfile, and builds
// on inspect miss (§6.1). Registry failure falls back to the newest existing
// image with a matching kekkai.config_hash label (§6.2). The second return is
// the resolved claude version — empty on the fallback path (version unknown),
// which gates the sandbox-context injection (§5.3).
func ensureImage(cfg *config.Config, verbose bool) (string, string, error) {
	aptPackages := append(append([]string{}, builtinAptPackages...), cfg.Image.AptPackages...)
	baseImage := cfg.Image.ResolvedBaseImage()
	configHash := ConfigHash(baseImage, aptPackages, assets.FirewallScript)

	version := cfg.Claude.Version
	if version == "latest" {
		resolved, err := resolveLatest()
		if err != nil {
			tag, found := newestImageForConfig(configHash)
			if !found {
				return "", "", fmt.Errorf("could not resolve latest claude version (%v) and no existing kekkai image matches this config — retry online or pin claude.version", err)
			}
			fmt.Fprintf(os.Stderr, "warning: npm registry unreachable (%v), reusing existing image %s\n", err, tag)
			return tag, "", nil
		}
		version = resolved
	}

	rendered, err := renderDockerfile(baseImage, aptPackages, version)
	if err != nil {
		return "", "", err
	}
	tag := ImageTag(rendered, assets.FirewallScript)
	if !docker.ImageExists(tag) {
		// The build will pull the base image; confirm the tag exists first so
		// a bad node_version fails fast with a config-shaped error (§6.1).
		// Best-effort only: skipped when the base image is already local, and
		// an unreachable registry falls through to the pull error.
		if !docker.ImageExists(baseImage) && baseImageMissing(baseImage) {
			return "", "", fmt.Errorf(
				"image.node_version: %q has no published base image (%s not found on Docker Hub) — pick a version from https://hub.docker.com/_/node",
				cfg.Image.NodeVersion, baseImage)
		}
		fmt.Printf("building image %s (claude %s)\n", tag, version)
		if err := buildImage(tag, rendered, configHash, verbose); err != nil {
			return "", "", err
		}
	}
	return tag, version, nil
}

// baseImageMissing reports whether Docker Hub CONFIRMS the node tag does not
// exist (HTTP 404). Any other outcome — timeout, transport error, unexpected
// status — is inconclusive and returns false so offline use never blocks.
func baseImageMissing(baseImage string) bool {
	tag := strings.TrimPrefix(baseImage, "node:")
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get("https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/node:pull")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var auth struct {
		Token string `json:"token"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&auth) != nil || auth.Token == "" {
		return false
	}

	req, err := http.NewRequest(http.MethodHead,
		"https://registry-1.docker.io/v2/library/node/manifests/"+tag, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+auth.Token)
	req.Header.Set("Accept",
		"application/vnd.docker.distribution.manifest.v2+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.index.v1+json, application/vnd.oci.image.manifest.v1+json")
	manifest, err := client.Do(req)
	if err != nil {
		return false
	}
	defer manifest.Body.Close()
	return manifest.StatusCode == http.StatusNotFound
}

func resolveLatest() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(npmLatestURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("npm registry returned %s", resp.Status)
	}
	var doc struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", err
	}
	if doc.Version == "" {
		return "", fmt.Errorf("npm registry response had no version")
	}
	return doc.Version, nil
}

func newestImageForConfig(configHash string) (string, bool) {
	images, err := docker.KekkaiImages()
	if err != nil {
		return "", false
	}
	for _, img := range images { // newest first
		if img.ConfigHash == configHash {
			return img.Tag, true
		}
	}
	return "", false
}

func renderDockerfile(baseImage string, aptPackages []string, claudeVersion string) (string, error) {
	tmpl, err := template.New("Dockerfile").Parse(assets.DockerfileTmpl)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	err = tmpl.Execute(&out, struct {
		BaseImage     string
		AptPackages   []string
		ClaudeVersion string
	}{baseImage, aptPackages, claudeVersion})
	return out.String(), err
}

func buildImage(tag, renderedDockerfile, configHash string, verbose bool) error {
	dir, err := os.MkdirTemp("", "kekkai-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(renderedDockerfile), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "init-firewall.sh"), []byte(assets.FirewallScript), 0o755); err != nil {
		return err
	}
	labels := map[string]string{LabelConfigHash: configHash}
	return docker.BuildImage(tag, dir, labels, verbose)
}

// buildRunArgs assembles `docker run` args in the §7.3 order: caps → builtin
// mounts → git mounts → disk.mounts → secrets shadows → builtin env → user
// env → firewall env (authoritative) → CLAUDE_ARGS → limits → workdir.
// claudeVersion gates the sandbox-context injection (§5.3); empty = unknown.
func buildRunArgs(cfg *config.Config, pwd, imageTag, claudeVersion string, opts UpOptions) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	args := []string{"run", "--rm", "-it",
		"--name", ContainerName(pwd),
		"--label", LabelCwd + "=" + pwd,
		"--label", LabelImageHash + "=" + strings.TrimPrefix(imageTag, "kekkai:"),
		"--label", LabelVersion + "=" + opts.Version,
		"--cap-add", "NET_ADMIN",
		"--cap-add", "NET_RAW",
	}

	// Builtin mounts (§5.2)
	args = append(args, "-v", pwd+":/workspace")
	claudeDir := filepath.Join(home, ".claude")
	// Pre-create so docker does not create it root-owned on first run.
	if err := os.MkdirAll(claudeDir, 0o700); err != nil {
		return nil, err
	}
	args = append(args, "-v", claudeDir+":/home/kekkai/.claude")
	args = append(args, "-v", HistoryVolume(pwd)+":/commandhistory")

	// Git mounts (§5.2)
	if cfg.Git.Enabled {
		gitconfig := filepath.Join(home, ".gitconfig")
		if _, err := os.Stat(gitconfig); err == nil {
			args = append(args, "-v", gitconfig+":/home/kekkai/.gitconfig:ro")
		} else {
			fmt.Fprintln(os.Stderr, "warning: git.enabled is true but ~/.gitconfig does not exist")
		}
	} else {
		// Enforceable no-commit: .git read-only, and without SYS_ADMIN the
		// agent cannot remount it (§5.2).
		gitDir := filepath.Join(pwd, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			args = append(args, "-v", gitDir+":/workspace/.git:ro")
		}
	}
	if cfg.Git.SSHAgent {
		hostSock := os.Getenv("SSH_AUTH_SOCK")
		if goruntime.GOOS == "darwin" {
			// A Mac host socket cannot cross the VM boundary; every
			// recognized runtime forwards the agent at this VM path (§5.2).
			hostSock = darwinAgentSocket
		}
		args = append(args, "-v", hostSock+":/ssh-agent")
		signers := filepath.Join(home, ".config", "git", "allowed_signers")
		if _, err := os.Stat(signers); err == nil {
			args = append(args, "-v", signers+":/home/kekkai/.config/git/allowed_signers:ro")
		}
	}

	// User mounts (§4.3): missing source is skip+notice when optional,
	// warn+skip otherwise — docker must not create host artifacts.
	for _, m := range cfg.Disk.Mounts {
		if m.Skip {
			fmt.Printf("notice: skipping optional mount %s (unset variable)\n", m.Source)
			continue
		}
		if _, err := os.Stat(m.HostPath); err != nil {
			if m.Optional {
				fmt.Printf("notice: skipping optional mount %s (source missing)\n", m.Source)
			} else {
				fmt.Fprintf(os.Stderr, "warning: mount source %s does not exist, skipping\n", m.HostPath)
			}
			continue
		}
		spec := m.HostPath + ":" + m.ContainerPath
		if m.ReadOnly {
			spec += ":ro"
		}
		args = append(args, "-v", spec)
	}

	// Secrets shadows (§8): stat-gated on the host before run.
	for _, rel := range cfg.Secrets.Hide {
		hostPath := filepath.Join(pwd, rel)
		containerPath := "/workspace/" + strings.TrimPrefix(rel, "/")
		info, err := os.Stat(hostPath)
		switch {
		case err != nil:
			fmt.Fprintf(os.Stderr, "warning: secrets.hide path %s does not exist, skipping\n", rel)
		case info.IsDir():
			args = append(args, "--tmpfs", containerPath)
		default:
			args = append(args, "-v", "/dev/null:"+containerPath+":ro")
		}
	}

	// Env (§5.3, §7.3): builtin → user → firewall (authoritative) → CLAUDE_ARGS
	addEnv := func(k, v string) { args = append(args, "-e", k+"="+v) }
	addEnv("CLAUDE_CONFIG_DIR", "/home/kekkai/.claude")
	addEnv("NODE_OPTIONS", "--max-old-space-size=4096")
	addEnv("POWERLEVEL9K_DISABLE_GITSTATUS", "true")
	addEnv("WORKSPACE", filepath.Base(pwd))
	// No telemetry/error-reporting/auto-update traffic (§5.3); the in-image
	// claude version is the update path. User env below can override.
	addEnv("CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC", "1")
	// Sandbox awareness (§5.3): marker always; prompt only when the resolved
	// claude supports --append-system-prompt interactively (specs/011).
	addEnv("KEKKAI_SANDBOX", "1")
	if supportsAppendPrompt(claudeVersion) {
		addEnv("KEKKAI_SYSTEM_PROMPT", sandboxPromptFor(cfg))
	} else {
		v := claudeVersion
		if v == "" {
			v = "version unknown"
		}
		fmt.Fprintln(os.Stderr, yellow(os.Stderr, fmt.Sprintf(
			"warning: claude %s does not support sandbox context injection (needs >= %s), starting without it",
			v, appendPromptMinVersion)))
	}
	userKeys := make([]string, 0, len(cfg.Env))
	for k := range cfg.Env {
		userKeys = append(userKeys, k)
	}
	sort.Strings(userKeys)
	for _, k := range userKeys {
		addEnv(k, cfg.Env[k])
	}
	if cfg.Git.SSHAgent {
		addEnv("SSH_AUTH_SOCK", "/ssh-agent")
	}
	if cfg.Network.AllowAll {
		addEnv("ALLOW_ALL", "1")
	}
	if cfg.Network.AllowGithub {
		addEnv("ALLOW_GITHUB", "1")
	}
	if len(cfg.Network.AllowedDomains) > 0 {
		addEnv("ALLOWED_DOMAINS", strings.Join(cfg.Network.AllowedDomains, " "))
	}
	if len(cfg.Network.AllowedCIDRs) > 0 {
		addEnv("ALLOWED_CIDRS", strings.Join(cfg.Network.AllowedCIDRs, " "))
	}
	claudeArgs := cfg.Claude.Args
	if len(opts.ExtraClaudeArgs) > 0 {
		claudeArgs += " " + strings.Join(opts.ExtraClaudeArgs, " ")
	}
	addEnv("CLAUDE_ARGS", claudeArgs)

	// Limits
	if cfg.Limits.CPUs > 0 {
		args = append(args, "--cpus", strconv.FormatFloat(cfg.Limits.CPUs, 'f', -1, 64))
	}
	if cfg.Limits.Memory != "" {
		args = append(args, "--memory", cfg.Limits.Memory)
	}

	args = append(args, "-w", "/workspace", imageTag)
	return args, nil
}
