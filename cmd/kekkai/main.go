package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/filidorwiese/kekkai/internal/runtime"
)

var version = "dev"

const usage = `kekkai — sandboxed Claude Code container per project folder

Usage:
  kekkai <command> [flags]

Commands:
  up        Build (if needed) and start the sandbox for $PWD, then exec claude.
  down      Stop and remove the sandbox container for $PWD.
  shell     Open zsh inside the running sandbox for $PWD.
  ps        List running kekkai containers.
  prune     Remove orphan containers + unused kekkai:* images.
  config    Show and validate the merged sandbox configuration for $PWD.
  doctor    Diagnose host setup.
  version   Print kekkai version.
  help      Show this help.

Run "kekkai <command> -h" for command-specific flags.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd := os.Args[1]
	rest := os.Args[2:]

	cwd, err := os.Getwd()
	if err != nil {
		exitErr(err)
	}

	switch cmd {
	case "up":
		os.Exit(runUp(cwd, rest))
	case "down":
		if err := runtime.Down(cwd); err != nil {
			exitErr(err)
		}
	case "shell":
		code, err := runtime.Shell(cwd)
		if err != nil {
			exitErr(err)
		}
		os.Exit(code)
	case "ps":
		if err := runtime.Ps(); err != nil {
			exitErr(err)
		}
	case "prune":
		os.Exit(runPrune(rest))
	case "config":
		os.Exit(runConfig(cwd, rest))
	case "doctor":
		code, err := runtime.Doctor(cwd)
		if err != nil {
			exitErr(err)
		}
		os.Exit(code)
	case "version", "--version", "-v":
		fmt.Println(version)
	case "help", "--help", "-h":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "kekkai: unknown command %q\n\n", cmd)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}

func runUp(cwd string, args []string) int {
	fs := flag.NewFlagSet("up", flag.ExitOnError)
	force := fs.Bool("force", false, "recreate the container if one already exists for this folder")
	verbose := fs.Bool("verbose", false, "use plain buildkit progress output")

	// split args at "--"
	var preArgs, extra []string
	split := false
	for _, a := range args {
		if !split && a == "--" {
			split = true
			continue
		}
		if split {
			extra = append(extra, a)
		} else {
			preArgs = append(preArgs, a)
		}
	}
	if err := fs.Parse(preArgs); err != nil {
		exitErr(err)
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "kekkai up: unexpected positional argument %q (use `-- %s` to forward to claude)\n", fs.Arg(0), fs.Arg(0))
		os.Exit(2)
	}

	opts := runtime.UpOptions{
		Force:       *force,
		Verbose:     *verbose,
		ExtraClaude: extra,
		Version:     version,
	}
	code, err := runtime.Up(cwd, opts)
	if err != nil {
		exitErr(err)
	}
	return code
}

func runConfig(cwd string, args []string) int {
	fs := flag.NewFlagSet("config", flag.ExitOnError)
	asYAML := fs.Bool("yaml", false, "output the merged config as a valid .kekkai.yaml document")
	if err := fs.Parse(args); err != nil {
		exitErr(err)
	}
	if err := runtime.Config(cwd, *asYAML); err != nil {
		exitErr(err)
	}
	return 0
}

func runPrune(args []string) int {
	fs := flag.NewFlagSet("prune", flag.ExitOnError)
	volumes := fs.Bool("volumes", false, "also remove orphan kekkai-history-* volumes")
	yes := fs.Bool("yes", false, "skip confirmation prompt")
	if err := fs.Parse(args); err != nil {
		exitErr(err)
	}
	if err := runtime.Prune(runtime.PruneOptions{Volumes: *volumes, Yes: *yes}); err != nil {
		exitErr(err)
	}
	return 0
}

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "kekkai: %v\n", err)
	os.Exit(1)
}
