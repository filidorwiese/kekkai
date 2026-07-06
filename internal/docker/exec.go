package docker

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// Interactive runs `docker <args>` with stdio attached, forwarding SIGINT and
// SIGTERM to the child so `docker run --rm -it` tears the container down on
// either signal (§7.2, research.md R9). Returns the child's exit code.
func Interactive(args ...string) (int, error) {
	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	if err := cmd.Start(); err != nil {
		return 1, err
	}
	done := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-sigCh:
				// docker CLI forwards the signal to the container process;
				// --rm then guarantees removal on exit.
				_ = cmd.Process.Signal(sig)
			case <-done:
				return
			}
		}
	}()

	err := cmd.Wait()
	close(done)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}
