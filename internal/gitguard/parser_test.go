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
	intent := Parse([]string{"checkout", "-b", "feature/auth"})
	if intent.TargetBranch != "feature/auth" {
		t.Fatalf("target branch = %q, want feature/auth", intent.TargetBranch)
	}
}
