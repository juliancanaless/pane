package gitguard

import (
	"fmt"

	"github.com/juliancanalez/pane/internal/session"
)

type PreflightInput struct {
	Intent           Intent
	CurrentSession   session.Session
	ActiveSessions   []session.Session
	FileOverlaps     []FileOverlap
	SemanticOverlaps []SemanticOverlap
}

type FileOverlap struct {
	PeerSessionID string
	SharedFiles   []string
}

type SemanticOverlap struct {
	PeerSessionID string
	ChangedFile   string
	DependentFile string
	Symbol        string
	Dependency    string
	Confidence    float64
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
	warnings = append(warnings, semanticOverlapWarnings(input)...)
	warnings = append(warnings, commandRiskWarning(input)...)
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

func semanticOverlapWarnings(input PreflightInput) []string {
	var warnings []string
	for _, overlap := range input.SemanticOverlaps {
		symbol := overlap.Symbol
		if symbol == "" {
			symbol = overlap.Dependency
		}
		warnings = append(warnings, fmt.Sprintf("session %s has recent work in %s, which depends on %s changed by your work in %s", overlap.PeerSessionID, overlap.DependentFile, symbol, overlap.ChangedFile))
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

func commandRiskWarning(input PreflightInput) []string {
	if !sameBranchPeer(input) && len(input.FileOverlaps) == 0 && len(input.SemanticOverlaps) == 0 {
		return nil
	}
	var risk string
	switch input.Intent.Subcommand {
	case "rebase":
		risk = "rebase rewrites commit history — this may disrupt other sessions on this branch"
	case "merge":
		risk = "merge modifies the branch head — other sessions may need to integrate these changes"
	case "reset":
		risk = "reset --hard discards local changes — this may destroy uncommitted work visible to other sessions"
	case "push":
		if input.Intent.Forceful {
			risk = "force push rewrites remote history — other sessions pulling this branch will diverge"
		}
	case "checkout", "switch":
		if !input.Intent.CreatingBranch && sameBranchPeer(input) {
			risk = "switching to a branch where another session is actively working"
		}
	}
	if risk == "" {
		return nil
	}
	return []string{risk}
}
