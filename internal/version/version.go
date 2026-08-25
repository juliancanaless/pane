package version

import (
	"strconv"
	"strings"
)

// Version is the pane release version, bumped as part of each release commit.
// The daemon stamps it on every response so the CLI can detect a stale daemon
// left running by a previous install and restart it with the new binary.
const Version = "0.1.10"

// IsOlder reports whether candidate is an older release than reference.
// An empty or unparseable candidate is treated as older: daemons predating
// version stamping never report one, and they are exactly the daemons that
// need replacing.
func IsOlder(candidate, reference string) bool {
	if candidate == reference {
		return false
	}
	candidateParts, err := parse(candidate)
	if err != nil {
		return true
	}
	referenceParts, err := parse(reference)
	if err != nil {
		return false
	}
	for i := range referenceParts {
		if candidateParts[i] != referenceParts[i] {
			return candidateParts[i] < referenceParts[i]
		}
	}
	return false
}

func parse(value string) ([3]int, error) {
	var parts [3]int
	fields := strings.SplitN(strings.TrimPrefix(strings.TrimSpace(value), "v"), ".", 3)
	for i := range parts {
		if i >= len(fields) {
			break
		}
		number, err := strconv.Atoi(fields[i])
		if err != nil {
			return parts, err
		}
		parts[i] = number
	}
	return parts, nil
}
