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
	// nvm dependency + kekkai shell (present in debian:trixie; listed to pin it)
	"bash",
	// subcommands
	"tcpdump", // kekkai traffic (nflog reader)
	// git over ssh: signing (ssh-keygen -Y) + git@ remotes via the
	// forwarded agent (§5.2); baked unconditionally per §6.1
	"openssh-client",
	// convenience
	"git", "gh", "less", "nano", "procps",
	// general tooling (specs/021): archives, file inspection, search, file
	// transfer — near-universal agent operations, zero config required
	"unzip", "zip", "xz-utils", "zstd", "bzip2",
	"file", "ripgrep", "fd-find", "rsync",
	// scripting runtime (specs/024): ad-hoc agent scripting; PyPI egress
	// stays user opt-in via network.allowed_domains (§5.4 unchanged)
	"python3", "python3-venv", "python3-pip",
}

const npmLatestURL = "https://registry.npmjs.org/@anthropic-ai/claude-code/latest"

// nodeIndexURL is nvm's source of truth: the same dataset (index.tab/json in
// the same dist directory) that `nvm install` resolves versions against, so
// the pre-build check can never disagree with what the build would do.
const nodeIndexURL = "https://nodejs.org/dist/index.json"

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
	pwd, err := ProjectDir()
	if err != nil {
		return 1, err
	}
	claudeDir, err := ClaudeConfigDir()
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
		// The project path is mirrored into the sandbox (specs/026); paths
		// that cannot be, join the same one-pass report (§4.4).
		errs = append(errs, ValidateProjectPath(pwd)...)
		// The Claude config dir is mirrored the same way (specs/028): same
		// path rules, plus no user mount may land on or under it — docker
		// would accept a descendant silently and hide part of the plugin
		// cache. Deliberately not in protectedContainerPaths: a project at
		// $HOME legitimately contains the config dir (research R3).
		errs = append(errs, ValidateClaudeConfigDir(claudeDir)...)
		for i, m := range cfg.Disk.Mounts {
			if !m.Skip && isAncestorOrSelf(claudeDir, m.ContainerPath) {
				errs = append(errs, fmt.Errorf("disk.mounts[%d]: target %s would shadow the Claude config dir %s", i, m.ContainerPath, claudeDir))
			}
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
	if err := preflight(cfg, pwd, claudeDir, imageTag); err != nil {
		return 1, err
	}

	args, cleanup, err := buildRunArgs(cfg, pwd, claudeDir, imageTag, claudeVersion, opts)
	if err != nil {
		return 1, err
	}
	// Removes the staged config placeholder (if any) once the container exits;
	// Interactive waits, so the bind source lives exactly as long as the sandbox.
	defer cleanup()

	select {
	case msg := <-noticeCh:
		if msg != "" {
			fmt.Println(Yellow(os.Stdout, msg))
		}
	default:
		// Check not finished — silent this run, goroutine abandoned.
	}
	return docker.Interactive(args...)
}

// Yellow wraps msg in the advisory yellow only when f is a terminal and
// NO_COLOR is unset (https://no-color.org). Every advisory line (missing
// config, update notices, sandbox-context warning) goes through here so
// the convention cannot diverge.
func Yellow(f *os.File, msg string) string {
	return paint(colorEnabled(f), ansiYellow, msg)
}

// colorEnabled is the one TTY + NO_COLOR decision shared by every colored
// surface (advisories here, the mpr transcript). Char-device check, not
// isatty: stdout to /dev/null is the only false positive and it is harmless.
func colorEnabled(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0 && os.Getenv("NO_COLOR") == ""
}

// SGR codes used by kekkai output; paint is a no-op when color is off.
const (
	ansiRed      = "31"
	ansiGreen    = "32"
	ansiYellow   = "33"
	ansiBlue     = "34"
	ansiMagenta  = "35"
	ansiCyanBold = "1;36"
)

func paint(enabled bool, code, msg string) string {
	if !enabled {
		return msg
	}
	return "\033[" + code + "m" + msg + "\033[0m"
}

// warnNoConfig prints the missing-config advisory (contract): one stderr line.
func warnNoConfig() {
	fmt.Fprintln(os.Stderr, Yellow(os.Stderr,
		"warning: no .kekkai.yaml found, using defaults - run 'kekkai init' to customize"))
}

// ensureImage resolves the claude version, renders the Dockerfile, and builds
// on inspect miss (§6.1). Registry failure falls back to the newest existing
// image with a matching kekkai.config_hash label (§6.2). The second return is
// the resolved claude version — empty on the fallback path (version unknown),
// which gates the sandbox-context injection (§5.3).
func ensureImage(cfg *config.Config, verbose bool) (string, string, error) {
	aptPackages := append(append([]string{}, builtinAptPackages...), cfg.Image.AptPackages...)
	uid, gid := sandboxIdentity()
	configHash := ConfigHash(cfg.Image.NodeVersion, aptPackages, cfg.Image.AptRepos, assets.FirewallScript, assets.MprScript, uid, gid)

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

	rendered, err := renderDockerfile(cfg.Image, aptPackages, version, uid, gid)
	if err != nil {
		return "", "", err
	}
	tag := ImageTag(rendered, assets.FirewallScript, assets.MprScript)
	if !docker.ImageExists(tag) {
		// Fail fast on a nonexistent node_version before the multi-minute
		// build (§6.1). Best-effort only: runs solely on a build-triggering
		// up (cached images never hit the network), and only a confirmed
		// absence aborts — lts always exists by construction.
		if cfg.Image.NodeVersion != config.DefaultNodeVersion && nodeVersionMissing(cfg.Image.NodeVersion) {
			return "", "", fmt.Errorf(
				"image.node_version: %q matches no published Node version — see https://nodejs.org/dist/ for available versions",
				cfg.Image.NodeVersion)
		}
		fmt.Printf("building image %s (claude %s)\n", tag, version)
		if err := buildImage(tag, rendered, configHash, cfg.Image.AptRepos, verbose); err != nil {
			return "", "", err
		}
	}
	return tag, version, nil
}

// nodeVersionMissing reports whether the Node release index CONFIRMS the
// selector matches no published version. Matching mirrors nvm's remote
// resolution: full pins match exactly, major/major.minor match any release
// under them. Any other outcome — timeout, transport error, non-200,
// malformed data — is inconclusive and returns false so a degraded network
// never blocks a build (same tri-state semantics as the retired node:*
// Docker Hub probe); nvm's own error remains the in-build fallback.
func nodeVersionMissing(selector string) bool {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(nodeIndexURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var releases []struct {
		Version string `json:"version"`
	}
	if json.NewDecoder(resp.Body).Decode(&releases) != nil || len(releases) == 0 {
		return false
	}
	exact := "v" + selector
	prefix := exact + "."
	for _, r := range releases {
		if r.Version == exact || strings.HasPrefix(r.Version, prefix) {
			return false
		}
	}
	return true
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

// aptRepoRender is the per-entry template data for the {{range .AptRepos}}
// block: paths and option string are derived here so the template stays a
// dumb interpolator (specs/020 contracts/image-render.md). The kekkai-
// filename prefix guarantees no collision with the builtin github-cli files.
type aptRepoRender struct {
	URL         string
	KeyURL      string
	KeyringPath string // empty when KeyURL is unset
	SourcesPath string
	Options     string
	SuiteLine   string
}

func aptRepoRenderData(repos []config.AptRepo) []aptRepoRender {
	out := make([]aptRepoRender, 0, len(repos))
	for _, r := range repos {
		d := aptRepoRender{
			URL:         r.URL,
			KeyURL:      r.KeyURL,
			SourcesPath: "/etc/apt/sources.list.d/kekkai-" + r.Name + ".list",
			Options:     "arch=$(dpkg --print-architecture)",
		}
		if r.KeyURL != "" {
			d.KeyringPath = "/etc/apt/keyrings/kekkai-" + r.Name + ".gpg"
			d.Options += " signed-by=" + d.KeyringPath
		}
		if strings.HasSuffix(r.Suite, "/") {
			// Flat repository: suite alone, no components token.
			d.SuiteLine = r.Suite
		} else if r.Components == "" {
			d.SuiteLine = r.Suite + " main"
		} else {
			d.SuiteLine = r.Suite + " " + r.Components
		}
		out = append(out, d)
	}
	return out
}

func renderDockerfile(img config.ImageConfig, aptPackages []string, claudeVersion string, uid, gid int) (string, error) {
	tmpl, err := template.New("Dockerfile").Parse(assets.DockerfileTmpl)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	err = tmpl.Execute(&out, struct {
		DebianImage    string
		NvmVersion     string
		NodeInstallArg string
		AptPackages    []string
		AptRepos       []aptRepoRender
		ClaudeVersion  string
		Uid            int
		Gid            int
		MprBaseURL     string
	}{config.DebianBaseImage, config.NvmVersion, img.NodeInstallArg(),
		aptPackages, aptRepoRenderData(img.AptRepos), claudeVersion, uid, gid,
		config.MprBaseURL})
	return out.String(), err
}

func buildImage(tag, renderedDockerfile, configHash string, aptRepos []config.AptRepo, verbose bool) error {
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
	if err := os.WriteFile(filepath.Join(dir, "kekkai-mpr.py"), []byte(assets.MprScript), 0o755); err != nil {
		return err
	}
	labels := map[string]string{LabelConfigHash: configHash}
	output, err := docker.BuildImage(tag, dir, labels, verbose, len(aptRepos) > 0)
	if err != nil {
		// Never replaces the build error — one extra stderr line at most.
		if hint := aptSignatureHint(aptRepos, output); hint != "" {
			fmt.Fprintln(os.Stderr, hint)
		}
		return err
	}
	return nil
}

// aptSignatureMarkers are apt's release-file verification failures; any of
// them in a failed build's output triggers the key_url hint (specs/020 R7).
var aptSignatureMarkers = []string{"NO_PUBKEY", "is not signed", "EXPKEYSIG", "NODATA"}

// aptSignatureHint attributes an apt signature failure to a configured repo
// by URL (apt's error lines carry the source URL) and suggests the key_url
// fix. Empty when apt_repos is unconfigured or no marker matched.
func aptSignatureHint(repos []config.AptRepo, buildOutput string) string {
	if len(repos) == 0 {
		return ""
	}
	matched := false
	for _, m := range aptSignatureMarkers {
		if strings.Contains(buildOutput, m) {
			matched = true
			break
		}
	}
	if !matched {
		return ""
	}
	for i, r := range repos {
		if strings.Contains(buildOutput, r.URL) {
			if r.KeyURL != "" {
				return fmt.Sprintf("hint: image.apt_repos[%d] (%s): apt could not verify this repository — key_url may point at the wrong key", i, r.Name)
			}
			return fmt.Sprintf("hint: image.apt_repos[%d] (%s): apt could not verify this repository — add key_url with the repository's signing key", i, r.Name)
		}
	}
	names := make([]string, len(repos))
	for i, r := range repos {
		names[i] = r.Name
	}
	return fmt.Sprintf("hint: an apt repository failed signature verification — check key_url on image.apt_repos entries (%s)", strings.Join(names, ", "))
}

// buildRunArgs assembles `docker run` args in the §7.3 order: caps → builtin
// mounts → git mounts → disk.mounts → secrets shadows → builtin env → user
// env → firewall env (authoritative) → CLAUDE_ARGS → limits → workdir (the
// mirrored project path, specs/026).
// claudeVersion gates the sandbox-context injection (§5.3); empty = unknown.
// The returned cleanup (never nil) releases the staged config placeholder and
// must run after the container exits.
func buildRunArgs(cfg *config.Config, pwd, claudeDir, imageTag, claudeVersion string, opts UpOptions) ([]string, func(), error) {
	cleanup := func() {}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, cleanup, err
	}

	// --init: claude (node) as PID 1 never reaps the orphaned grandchildren
	// of statusline/hook helpers, so they pile up as zombies for the whole
	// session. docker-init (tini) as PID 1 reaps them, forwards SIGTERM and
	// exits with claude's status; claude stays the tty foreground process so
	// Ctrl-C and SIGWINCH reach it directly. Run arg, not a bake-time input:
	// the image hash is unchanged (specs/027).
	args := []string{"run", "--rm", "-it", "--init",
		"--name", ContainerName(pwd),
		"--label", LabelCwd + "=" + pwd,
		"--label", LabelImageHash + "=" + strings.TrimPrefix(imageTag, "kekkai:"),
		"--label", LabelVersion + "=" + opts.Version,
		"--cap-add", "NET_ADMIN",
		"--cap-add", "NET_RAW",
	}

	// Builtin mounts (§5.2): the project is mirrored at its host path
	// (specs/026) so Claude keys per-project state identically on both sides.
	args = append(args, "-v", pwd+":"+pwd)
	// Config visibility (§5.2, specs/012): the config — or a comment-only
	// placeholder when absent — is bound read-only over the rw project bind,
	// so the agent can read the active policy but never rewrite the file that
	// governs its own sandbox (no SYS_ADMIN → no remount, same enforcement as
	// the .git ro bind). Load already rejected non-regular entries.
	configPath := filepath.Join(pwd, ".kekkai.yaml")
	if info, err := os.Stat(configPath); err == nil && info.Mode().IsRegular() {
		args = append(args, "-v", configPath+":"+configPath+":ro")
	} else {
		placeholder, release, err := writeConfigPlaceholder()
		if err != nil {
			return nil, cleanup, err
		}
		// Docker materializes the mountpoint as an empty file inside the
		// project bind, i.e. on the host. Remove that remnant at exit —
		// but only an empty regular file, so a real config the user writes
		// while the sandbox runs is never touched (specs/012).
		cleanup = func() {
			release()
			if info, err := os.Stat(configPath); err == nil &&
				info.Mode().IsRegular() && info.Size() == 0 {
				os.Remove(configPath)
			}
		}
		args = append(args, "-v", placeholder+":"+configPath+":ro")
	}
	// The Claude config dir is mirrored at its host path too (specs/028):
	// Claude Code stores host-absolute paths under it (plugin installs,
	// marketplaces, hook commands), which only resolve when the dir sits at
	// the same path inside. /home/kekkai/.claude becomes a symlink to it at
	// container start (CMD). Pre-create so docker does not create it
	// root-owned on first run.
	if err := os.MkdirAll(claudeDir, 0o700); err != nil {
		return nil, cleanup, err
	}
	args = append(args, "-v", claudeDir+":"+claudeDir)
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
			args = append(args, "-v", gitDir+":"+gitDir+":ro")
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

	// Secrets shadows (§8): stat-gated on the host before run. Host and
	// container path coincide under the mirrored project path.
	for _, rel := range cfg.Secrets.Hide {
		path := filepath.Join(pwd, rel)
		info, err := os.Stat(path)
		switch {
		case err != nil:
			fmt.Fprintf(os.Stderr, "warning: secrets.hide path %s does not exist, skipping\n", rel)
		case info.IsDir():
			args = append(args, "--tmpfs", path)
		default:
			args = append(args, "-v", "/dev/null:"+path+":ro")
		}
	}

	// Env (§5.3, §7.3): builtin → user → firewall (authoritative) → CLAUDE_ARGS
	addEnv := func(k, v string) { args = append(args, "-e", k+"="+v) }
	// First so a user env entry can still override (research R6).
	addEnv("CLAUDE_CONFIG_DIR", claudeDir)
	addEnv("NODE_OPTIONS", "--max-old-space-size=4096")
	addEnv("WORKSPACE", filepath.Base(pwd))
	// No telemetry/error-reporting/auto-update traffic (§5.3); the in-image
	// claude version is the update path. User env below can override.
	addEnv("CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC", "1")
	// Sandbox awareness (§5.3): marker always; prompt only when the resolved
	// claude supports --append-system-prompt interactively (specs/011).
	addEnv("KEKKAI_SANDBOX", "1")
	// Model-provider capture (§5.3, specs/025): claude talks to the
	// in-sandbox loopback proxy. Deliberately before user env — a user
	// ANTHROPIC_BASE_URL wins by last-value and thereby disables capture.
	addEnv("ANTHROPIC_BASE_URL", config.MprBaseURL)
	if supportsAppendPrompt(claudeVersion) {
		addEnv("KEKKAI_SYSTEM_PROMPT", sandboxPromptFor(cfg))
	} else {
		v := claudeVersion
		if v == "" {
			v = "version unknown"
		}
		fmt.Fprintln(os.Stderr, Yellow(os.Stderr, fmt.Sprintf(
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

	args = append(args, "-w", pwd, imageTag)
	return args, cleanup, nil
}

// configPlaceholder is the in-container stand-in for a missing .kekkai.yaml.
// Comments-only means all defaults (§4.1), so the agent reading it sees
// exactly the active configuration. Exact text is contract-pinned
// (specs/012-readonly-config-mount/contracts/config-mount.md).
const configPlaceholder = "# no .kekkai.yaml in workspace - kekkai runs on defaults; create one on the host ('kekkai init') to customize\n"

// writeConfigPlaceholder stages the placeholder outside the project dir and
// returns its path plus a release func. It lives under the user cache dir,
// not os.TempDir: a bind source must be visible to the docker daemon, and on
// macOS only the home directory is shared into the runtime VM by every
// recognized runtime (colima does not share $TMPDIR).
func writeConfigPlaceholder() (string, func(), error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", nil, err
	}
	base := filepath.Join(cacheDir, "kekkai")
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp(base, "config-")
	if err != nil {
		return "", nil, err
	}
	path := filepath.Join(dir, ".kekkai.yaml")
	if err := os.WriteFile(path, []byte(configPlaceholder), 0o444); err != nil {
		os.RemoveAll(dir)
		return "", nil, err
	}
	return path, func() { os.RemoveAll(dir) }, nil
}
