package gitguard

import (
	"fmt"

	"github.com/juliancanalez/pane/internal/session"
)

type PreflightInput struct {
	Intent         Intent
	CurrentSession session.Session
	ActiveSessions []session.Session
	FileOverlaps   []FileOverlap
}

type FileOverlap struct {
	PeerSessionID string
	SharedFiles   []string
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
	warnings = append(warnings, fileOverlapWarnings(input)...)
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

func fileOverlapWarnings(input PreflightInput) []string {
	var warnings []string
	for _, overlap := range input.FileOverlaps {
		if len(overlap.SharedFiles) == 0 {
			continue
		}
		fileList := overlap.SharedFiles
		if len(fileList) > 3 {
			fileList = fileList[:3]
		}
		warnings = append(warnings, fmt.Sprintf("session %s has recent activity in overlapping files: %s", overlap.PeerSessionID, joinFiles(fileList, len(overlap.SharedFiles))))
	}
	return warnings
}

func joinFiles(files []string, total int) string {
	result := ""
	for i, f := range files {
		if i > 0 {
			result += ", "
		}
		result += f
	}
	if total > len(files) {
		result += fmt.Sprintf(" (+%d more)", total-len(files))
	}
	return result
}
