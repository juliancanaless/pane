package gitguard

import (
	"fmt"

	"github.com/juliancanalez/pane/internal/session"
)

type PreflightInput struct {
	Intent         Intent
	CurrentSession session.Session
	ActiveSessions []session.Session
}

type PreflightResult struct {
	Warnings []string
	Block    bool
}

func Preflight(input PreflightInput) PreflightResult {
	if !input.Intent.Watched {
		return PreflightResult{}
	}
	warnings := branchWarnings(input)
	block := input.Intent.Forceful && sameBranchPeer(input)
	if block {
		warnings = append(warnings, "forceful git operation while another session is active on this branch")
	}
	return PreflightResult{Warnings: warnings, Block: block}
}

func branchWarnings(input PreflightInput) []string {
	var warnings []string
	for _, peer := range input.ActiveSessions {
		if peer.ID == input.CurrentSession.ID {
			continue
		}
		if peer.Branch != "" && peer.Branch == input.CurrentSession.Branch {
			warnings = append(warnings, fmt.Sprintf("session %s is also active on branch %s", peer.ID, peer.Branch))
			continue
		}
		if input.Intent.TargetBranch != "" && peer.Branch == input.Intent.TargetBranch {
			warnings = append(warnings, fmt.Sprintf("session %s is active on target branch %s", peer.ID, peer.Branch))
		}
	}
	return warnings
}

func sameBranchPeer(input PreflightInput) bool {
	for _, peer := range input.ActiveSessions {
		if peer.ID != input.CurrentSession.ID && peer.Branch != "" && peer.Branch == input.CurrentSession.Branch {
			return true
		}
	}
	return false
}
