// Package selfupdate updates the running kekkai binary to the latest
// GitHub release: same artifacts and checksum manifest install.sh
// consumes, verified before extraction, atomic in-place replace (§10).
package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const defaultRepo = "filidorwiese/kekkai"

// Run updates the executable that is currently running to the latest
// release. version is the ldflags-injected main.version.
func Run(version string) error {
	// Dev builds have no matching release artifact (FR-007).
	if version == "dev" || !strings.HasPrefix(version, "v") {
		return errors.New("self-update is unavailable on dev builds; install a release: " +
			"curl -fsSL https://raw.githubusercontent.com/filidorwiese/kekkai/main/install.sh | sh")
	}

	target, err := binaryPath()
	if err != nil {
		return err
	}
	// Preflight before the first network byte (FR-008).
	if err := checkWritable(target); err != nil {
		return err
	}

	repo := repoSlug()
	latest, err := latestTag(repo)
	if err != nil {
		return err
	}

	// Equal or ahead returns before any download or temp file (SC-003).
	switch compareVersions(version, latest) {
	case 0:
		fmt.Printf("You're on the latest version (%s)\n", version)
		return nil
	case 1:
		fmt.Printf("You're ahead of the latest release (%s > %s)\n", version, latest)
		return nil
	}
	return update(repo, version, latest, target)
}

func repoSlug() string {
	// Same override install.sh honors; enables e2e against a fork.
	if r := os.Getenv("KEKKAI_REPO"); r != "" {
		return r
	}
	return defaultRepo
}

// binaryPath resolves the real on-disk location of the running binary,
// through symlinks, so the rename lands on the actual file.
func binaryPath() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return "", fmt.Errorf("cannot update: %v", err)
	}
	return exe, nil
}

// checkWritable probes the target binary and its directory without
// writing anything: SC-003 demands the up-to-date path stays
// side-effect free, so no probe files.
func checkWritable(target string) error {
	fail := func(path string) error {
		return fmt.Errorf("cannot update: %s is not writable; fix permissions or reinstall via install.sh", path)
	}
	// access(2), not open-for-write: opening the running executable
	// fails with ETXTBSY on Linux regardless of permissions.
	const wOK = 0x2
	if err := syscall.Access(target, wOK); err != nil {
		return fail(target)
	}
	// The rename needs write on the directory too (temp file + replace).
	dir := filepath.Dir(target)
	if err := syscall.Access(dir, wOK); err != nil {
		return fail(dir)
	}
	return nil
}

func latestTag(repo string) (string, error) {
	fail := func(cause any) (string, error) {
		return "", fmt.Errorf("could not determine the latest release of %s: %v", repo, cause)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		return fail(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return "", errors.New("GitHub API rate limit hit; try again later")
	}
	if resp.StatusCode != http.StatusOK {
		return fail(fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fail(err)
	}
	if release.TagName == "" {
		return fail("no tag_name in API response")
	}
	return release.TagName, nil
}

// compareVersions orders two vMAJOR.MINOR.PATCH strings by their
// numeric core (-1/0/+1); pre-release/build suffixes are ignored.
func compareVersions(a, b string) int {
	va, vb := parseSemver(a), parseSemver(b)
	for i := range va {
		if va[i] != vb[i] {
			if va[i] < vb[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func parseSemver(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	for i, part := range strings.SplitN(v, ".", 3) {
		n, err := strconv.Atoi(part)
		if err != nil {
			break
		}
		out[i] = n
	}
	return out
}

func update(repo, from, to, target string) error {
	binary, err := download(repo, to)
	if err != nil {
		return err
	}

	dir := filepath.Dir(target)
	removeStaleTempFiles(dir)

	// Temp file next to the target: same filesystem makes the rename
	// atomic; the running process keeps its inode.
	tmp := filepath.Join(dir, fmt.Sprintf(".kekkai-update-%d", os.Getpid()))
	if err := os.WriteFile(tmp, binary, 0o755); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("download failed: %v", err)
	}
	// Explicit chmod: WriteFile's mode is umask-filtered.
	if err := os.Chmod(tmp, 0o755); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("download failed: %v", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("download failed: %v", err)
	}
	fmt.Printf("Updated kekkai %s -> %s\n", from, to)
	return nil
}

// removeStaleTempFiles clears leftovers of interrupted earlier runs
// (temp names embed the pid, so they never match the current run).
func removeStaleTempFiles(dir string) {
	stale, _ := filepath.Glob(filepath.Join(dir, ".kekkai-update-*"))
	for _, path := range stale {
		os.Remove(path)
	}
}

// download fetches the platform tarball and SHA256SUMS for the tag,
// verifies the checksum BEFORE extraction, and returns the kekkai
// binary bytes. Nothing touches the filesystem here.
func download(repo, tag string) ([]byte, error) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	tarball := fmt.Sprintf("kekkai_%s_%s_%s.tar.gz", tag, goos, goarch)
	base := fmt.Sprintf("https://github.com/%s/releases/download/%s/", repo, tag)

	fmt.Printf("downloading kekkai %s (%s/%s)\n", tag, goos, goarch)

	sums, err := fetch(base+"SHA256SUMS", 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("download failed: %v", err)
	}
	data, err := fetch(base+tarball, 120*time.Second)
	if errors.Is(err, errNotFound) {
		return nil, fmt.Errorf("no %s/%s artifact in release %s", goos, goarch, tag)
	}
	if err != nil {
		return nil, fmt.Errorf("download failed: %v", err)
	}

	want := manifestSum(sums, tarball)
	got := fmt.Sprintf("%x", sha256.Sum256(data))
	if want == "" || want != got {
		return nil, fmt.Errorf("checksum verification FAILED for %s", tarball)
	}
	return extractBinary(data)
}

var errNotFound = errors.New("not found")

func fetch(url string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

// manifestSum finds the sha256 for name in sha256sum-format output
// ("<hex>  <file>" per line). Empty string when absent.
func manifestSum(manifest []byte, name string) string {
	for _, line := range strings.Split(string(manifest), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return fields[0]
		}
	}
	return ""
}

func extractBinary(tgz []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(tgz))
	if err != nil {
		return nil, fmt.Errorf("download failed: %v", err)
	}
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("download failed: %v", err)
		}
		if header.Typeflag == tar.TypeReg && filepath.Base(header.Name) == "kekkai" {
			return io.ReadAll(tr)
		}
	}
	return nil, errors.New("download failed: no kekkai binary in tarball")
}
