package gitguard

import "testing"

func TestParseWatchedGitCommands(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		watched bool
	}{
		{name: "commit", args: []string{"commit", "-m", "msg"}, watched: true},
		{name: "reset hard", args: []string{"reset", "--hard"}, watched: true},
		{name: "reset soft", args: []string{"reset", "--soft"}, watched: false},
		{name: "status", args: []string{"status"}, watched: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := Parse(tt.args)
			if intent.Watched != tt.watched {
				t.Fatalf("watched = %v, want %v", intent.Watched, tt.watched)
			}
		})
	}
}

func TestParseForcefulPush(t *testing.T) {
	intent := Parse([]string{"push", "--force-with-lease"})
	if !intent.Forceful {
		t.Fatal("expected forceful intent")
	}
}

func TestParseTargetBranch(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		targetBranch   string
		creatingBranch bool
	}{
		{name: "checkout existing", args: []string{"checkout", "feature/auth"}, targetBranch: "feature/auth", creatingBranch: false},
		{name: "checkout create", args: []string{"checkout", "-b", "feature/auth"}, targetBranch: "feature/auth", creatingBranch: true},
		{name: "switch existing", args: []string{"switch", "feature/auth"}, targetBranch: "feature/auth", creatingBranch: false},
		{name: "switch create", args: []string{"switch", "-c", "feature/new"}, targetBranch: "feature/new", creatingBranch: true},
		{name: "merge", args: []string{"merge", "feature/auth"}, targetBranch: "feature/auth", creatingBranch: false},
		{name: "rebase", args: []string{"rebase", "main"}, targetBranch: "main", creatingBranch: false},
		{name: "push with remote and branch", args: []string{"push", "origin", "main"}, targetBranch: "main", creatingBranch: false},
		{name: "push no branch", args: []string{"push"}, targetBranch: "", creatingBranch: false},
		{name: "push force with target", args: []string{"push", "--force", "origin", "main"}, targetBranch: "main", creatingBranch: false},
		{name: "status", args: []string{"status"}, targetBranch: "", creatingBranch: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := Parse(tt.args)
			if intent.TargetBranch != tt.targetBranch {
				t.Fatalf("target branch = %q, want %q", intent.TargetBranch, tt.targetBranch)
			}
			if intent.CreatingBranch != tt.creatingBranch {
				t.Fatalf("creating branch = %v, want %v", intent.CreatingBranch, tt.creatingBranch)
			}
		})
	}
}
