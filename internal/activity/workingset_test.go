package activity

import "testing"

func TestRecentFilesDeduplicates(t *testing.T) {
	got := RecentFiles([]FileActivity{{Path: "a.go"}, {Path: "a.go"}, {Path: "b.go"}}, 2)
	if len(got) != 2 || got[0] != "a.go" || got[1] != "b.go" {
		t.Fatalf("RecentFiles = %#v", got)
	}
}
