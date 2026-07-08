// kekkai runs Claude Code inside a per-project Docker sandbox.
// Dispatch only — subcommand logic lives in internal/runtime (§3).
package main

import (
	"flag"
	"fmt"
	"os"

	"kekkai/internal/runtime"
	"kekkai/internal/selfupdate"
)

// version is injected via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

const usage = `kekkai — run Claude Code in a per-project Docker sandbox

Usage: kekkai <command> [flags]

Commands:
  init        write starter .kekkai.yaml
  up          build image if needed, start sandbox, exec claude
              flags: --force (recreate existing container)
                     --verbose (plain buildkit progress)
              args after -- are appended to claude args
  down        stop + remove the sandbox container for $PWD
  shell       open zsh in the running sandbox for $PWD
  ps          list running kekkai containers
  prune       remove orphan containers + unused kekkai:* images
              flags: --volumes (include history volumes)
                     --yes (skip confirmation prompt)
  self-update update kekkai to the latest release
  version     print version
  help        show this help
`

func main() {
	os.Exit(dispatch(os.Args[1:]))
}

func dispatch(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 1
	}

	var err error
	code := 0
	switch args[0] {
	case "init":
		err = runtime.Init()
	case "up":
		code, err = upCommand(args[1:])
	case "down":
		err = runtime.Down()
	case "shell":
		code, err = runtime.Shell()
	case "ps":
		err = runtime.Ps()
	case "prune":
		err = pruneCommand(args[1:])
	case "self-update":
		if len(args) > 1 {
			err = fmt.Errorf("unexpected argument %q (self-update takes none)", args[1])
		} else {
			err = selfupdate.Run(version)
		}
	case "version":
		fmt.Println(version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "kekkai: unknown command %q\n\n%s", args[0], usage)
		return 1
	}
	if err != nil {
		// No prefix: contracts/cli.md pins exact error strings (e.g. the
		// missing-config message).
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return code
}

func upCommand(args []string) (int, error) {
	// Everything after -- goes verbatim to claude args; flags before it.
	flagArgs, claudeArgs := args, []string(nil)
	for i, a := range args {
		if a == "--" {
			flagArgs, claudeArgs = args[:i], args[i+1:]
			break
		}
	}
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	force := fs.Bool("force", false, "recreate an existing container")
	verbose := fs.Bool("verbose", false, "plain buildkit progress")
	if err := fs.Parse(flagArgs); err != nil {
		return 1, nil // flag package already printed the error
	}
	if len(fs.Args()) > 0 {
		return 1, fmt.Errorf("unexpected argument %q (claude args go after --)", fs.Args()[0])
	}
	return runtime.Up(runtime.UpOptions{
		Force:           *force,
		Verbose:         *verbose,
		ExtraClaudeArgs: claudeArgs,
		Version:         version,
	})
}

func pruneCommand(args []string) error {
	fs := flag.NewFlagSet("prune", flag.ContinueOnError)
	volumes := fs.Bool("volumes", false, "include history volumes")
	yes := fs.Bool("yes", false, "skip confirmation prompt")
	if err := fs.Parse(args); err != nil {
		return nil
	}
	return runtime.Prune(*volumes, *yes)
}
