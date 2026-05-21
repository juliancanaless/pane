package activity

import "testing"

func TestComputeOverlap(t *testing.T) {
	pathSessions := map[string][]string{
		"shared1.go": {"alice", "bob"},
		"shared2.go": {"alice", "bob"},
		"shared3.go": {"alice", "carol"},
		"solo.go":    {"alice"},
	}
	overlaps := ComputeOverlap(pathSessions)

	// alice-bob share 2 files, alice-carol share 1 → alice-bob should come first
	if len(overlaps) != 2 {
		t.Fatalf("expected 2 overlaps, got %d: %+v", len(overlaps), overlaps)
	}

	ab := overlaps[0]
	if ab.SessionA != "alice" || ab.SessionB != "bob" {
		t.Fatalf("expected alice-bob first, got %s-%s", ab.SessionA, ab.SessionB)
	}
	if len(ab.SharedFiles) != 2 || ab.SharedFiles[0] != "shared1.go" || ab.SharedFiles[1] != "shared2.go" {
		t.Fatalf("unexpected shared files for alice-bob: %v", ab.SharedFiles)
	}

	ac := overlaps[1]
	if ac.SessionA != "alice" || ac.SessionB != "carol" {
		t.Fatalf("expected alice-carol second, got %s-%s", ac.SessionA, ac.SessionB)
	}
	if len(ac.SharedFiles) != 1 || ac.SharedFiles[0] != "shared3.go" {
		t.Fatalf("unexpected shared files for alice-carol: %v", ac.SharedFiles)
	}
}

func TestHotDirectories(t *testing.T) {
	files := []string{
		"/workspace/src/auth/token.go",
		"/workspace/src/auth/session.go",
		"/workspace/src/auth/middleware.go",
		"/workspace/src/payments/stripe.go",
		"/workspace/tests/auth_test.go",
	}
	hot := HotDirectories(files, 3)
	if len(hot) != 1 || hot[0] != "/workspace/src/auth" {
		t.Fatalf("HotDirectories = %v, want [/workspace/src/auth]", hot)
	}
}

func TestRecentFilesDeduplicates(t *testing.T) {
	got := RecentFiles([]FileActivity{{Path: "a.go"}, {Path: "a.go"}, {Path: "b.go"}}, 2)
	if len(got) != 2 || got[0] != "a.go" || got[1] != "b.go" {
		t.Fatalf("RecentFiles = %#v", got)
	}
}
