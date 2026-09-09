package runtime

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"kekkai/internal/config"
	"kekkai/internal/docker"
)

// Mpr streams the sandbox's model-provider exchanges for $PWD (specs/025).
// The capture proxy is already running inside the container; this attaches
// one `kekkai-mpr follow` subscriber via docker exec, reassembles streamed
// responses, and renders the transcript (or --raw JSON lines). Observe-only:
// nothing here touches requests, the proxy, or container state.
func Mpr(raw bool) (int, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return 1, err
	}
	containers, err := docker.ContainersByLabel(LabelCwd + "=" + pwd)
	if err != nil {
		return 1, err
	}
	containerID := ""
	for _, c := range containers {
		if c.Running {
			containerID = c.ID
			break
		}
	}
	if containerID == "" {
		return 1, fmt.Errorf("no running sandbox for %s, run 'kekkai up'", pwd)
	}

	// The container env is the truth about capture: absent = started by a
	// kekkai without this feature; overridden = the user pointed claude
	// elsewhere, which disables capture by design (FR-018).
	env, err := docker.ContainerEnv(containerID)
	if err != nil {
		return 1, err
	}
	baseURL, found := "", false
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "ANTHROPIC_BASE_URL="); ok {
			baseURL, found = v, true // last value wins, like docker
		}
	}
	if !found {
		return 1, fmt.Errorf("sandbox predates 'kekkai mpr'; run 'kekkai down' and 'kekkai up' to rebuild")
	}
	if baseURL != config.MprBaseURL {
		return 1, fmt.Errorf("capture disabled: ANTHROPIC_BASE_URL is set in .kekkai.yaml")
	}

	fmt.Fprintf(os.Stderr, "watching model-provider requests of sandbox for %s (Ctrl+C to stop)\n", pwd)

	// A per-session token on the reader's argv lets cleanup pkill exactly
	// this reader and not another terminal's (FR-016).
	token := make([]byte, 6)
	if _, err := rand.Read(token); err != nil {
		return 1, err
	}
	readerArgv := []string{"kekkai-mpr", "follow", hex.EncodeToString(token)}

	cmd := exec.Command("docker", append([]string{"exec", containerID}, readerArgv...)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1, err
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return 1, err
	}

	lines := make(chan string, 16)
	exit := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		// One event line carries a whole body (capped at 16 MiB in the
		// proxy, JSON-escaped here), so the scanner must accept far more
		// than its 64 KiB default.
		sc.Buffer(make([]byte, 1<<20), 64<<20)
		for sc.Scan() {
			lines <- sc.Text()
		}
		exit <- cmd.Wait()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)

	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	r := mprRenderer{raw: raw, color: !raw && colorEnabled(os.Stdout)}
	for {
		select {
		case line := <-lines:
			var ev wireEvent
			if err := json.Unmarshal([]byte(line), &ev); err != nil {
				continue // never a wire event we know; the proxy owns the format
			}
			for _, l := range r.render(ev) {
				io.WriteString(out, l+"\n")
			}
			out.Flush()
		case <-sig:
			// docker CLI does not forward signals to exec'd processes
			// (feature 009): kill the reader explicitly, by its token.
			_ = cmd.Process.Kill()
			_ = exec.Command("docker", "exec", containerID,
				"pkill", "-f", strings.Join(readerArgv, " ")).Run()
			return 0, nil
		case err := <-exit:
			msg := strings.TrimSpace(stderr.String())
			switch {
			case exitCode(err) == 126 || exitCode(err) == 127:
				fmt.Fprintln(os.Stderr, "sandbox image predates 'kekkai mpr'; run 'kekkai down' and 'kekkai up' to rebuild")
			case strings.Contains(msg, "mpr proxy unavailable"):
				fmt.Fprintln(os.Stderr, "mpr proxy unavailable")
			default:
				fmt.Fprintln(os.Stderr, "sandbox stopped")
			}
			return 1, nil
		}
	}
}
