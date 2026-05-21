package gitguard

import "strings"

type Intent struct {
	Args           []string
	Subcommand     string
	Watched        bool
	Forceful       bool
	TargetBranch   string
	CreatingBranch bool
}

func Parse(args []string) Intent {
	intent := Intent{Args: append([]string(nil), args...)}
	if len(args) == 0 {
		return intent
	}

	intent.Subcommand = args[0]
	intent.Watched = isWatchedSubcommand(intent.Subcommand, args[1:])
	intent.Forceful = hasForceFlag(args[1:])
	branch, creating := targetBranch(intent.Subcommand, args[1:])
	intent.TargetBranch = branch
	intent.CreatingBranch = creating
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

func targetBranch(subcommand string, args []string) (string, bool) {
	switch subcommand {
	case "checkout":
		return parseCheckoutTarget(args)
	case "switch":
		return parseSwitchTarget(args)
	case "merge", "rebase":
		return firstNonFlag(args), false
	case "push":
		return parsePushTarget(args), false
	default:
		return "", false
	}
}

func parseCheckoutTarget(args []string) (string, bool) {
	creating := false
	for i, arg := range args {
		if arg == "-b" || arg == "-B" {
			creating = true
			if i+1 < len(args) {
				return args[i+1], creating
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg, creating
	}
	return "", creating
}

func parseSwitchTarget(args []string) (string, bool) {
	creating := false
	for i, arg := range args {
		if arg == "-c" || arg == "-C" {
			creating = true
			if i+1 < len(args) {
				return args[i+1], creating
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg, creating
	}
	return "", creating
}

func parsePushTarget(args []string) string {
	positional := make([]string, 0, 2)
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		positional = append(positional, arg)
		if len(positional) == 2 {
			break
		}
	}
	if len(positional) >= 2 {
		return positional[1]
	}
	return ""
}

func firstNonFlag(args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
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
