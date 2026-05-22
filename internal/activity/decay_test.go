package activity

import (
	"reflect"
	"testing"
	"time"
)

func TestDecayActivitiesBucketsByAge(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	items := []FileActivity{
		{Path: "internal/daemon/daemon.go", Timestamp: now.Add(-time.Minute).Unix()},
		{Path: "internal/daemon/daemon.go", Timestamp: now.Add(-2 * time.Minute).Unix()},
		{Path: "internal/store/analysis.go", Timestamp: now.Add(-30 * time.Minute).Unix()},
		{Path: "internal/store/sessions.go", Timestamp: now.Add(-90 * time.Minute).Unix()},
		{Path: "analysis/src/main.rs", Timestamp: now.Add(-3 * time.Hour).Unix()},
		{Path: "old/ignored.go", Timestamp: now.Add(-80 * time.Hour).Unix()},
	}

	digest := DecayActivities(items, now, 3, 2)
	if !reflect.DeepEqual(digest.FullFiles, []string{"internal/daemon/daemon.go"}) {
		t.Fatalf("FullFiles = %#v", digest.FullFiles)
	}
	if digest.SummaryFiles != 2 || !reflect.DeepEqual(digest.SummaryDirs, []string{"internal/store"}) {
		t.Fatalf("summary digest = %#v", digest)
	}
	if digest.CompressedFiles != 1 || !reflect.DeepEqual(digest.CompressedDirs, []string{"analysis/src"}) {
		t.Fatalf("compressed digest = %#v", digest)
	}
}

func TestDecayDigestLines(t *testing.T) {
	digest := DecayDigest{SummaryFiles: 2, SummaryDirs: []string{"internal/store"}, CompressedFiles: 1, CompressedDirs: []string{"analysis/src"}}
	lines := digest.Lines()
	want := []string{
		"2 files in summary tier (5m–2h): internal/store",
		"1 file compressed (2h–72h): analysis/src",
	}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("Lines = %#v", lines)
	}
}
