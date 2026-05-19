package gitguard

import "strings"

type Intent struct {
	Args         []string
	Subcommand   string
	Watched      bool
	Forceful     bool
	TargetBranch string
}

func Parse(args []string) Intent {
	intent := Intent{Args: append([]string(nil), args...)}
	if len(args) == 0 {
		return intent
	}

	intent.Subcommand = args[0]
	intent.Watched = isWatchedSubcommand(intent.Subcommand, args[1:])
	intent.Forceful = hasForceFlag(args[1:])
	intent.TargetBranch = targetBranch(intent.Subcommand, args[1:])
	return intent
}

func isWatchedSubcommand(subcommand string, args []string) bool {
	switch subcommand {
	case "checkout", "switch", "commit", "pull", "push", "merge", "rebase":
		return true
	case "reset":
		return contains(args, "--hard")
	default:
		return false
	}
}

func hasForceFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--force" || arg == "-f" || strings.HasPrefix(arg, "--force-") {
			return true
		}
	}
	return false
}

func targetBranch(subcommand string, args []string) string {
	switch subcommand {
	case "checkout", "switch", "merge", "rebase":
		for _, arg := range args {
			if strings.HasPrefix(arg, "-") {
				continue
			}
			return arg
		}
	}
	return ""
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
