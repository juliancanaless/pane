package cli

import "testing"

func TestParseHistoryArgsRepoAndSince(t *testing.T) {
	since, repoScope, lineage, format, err := parseHistoryArgs([]string{"--repo", "--lineage", "--format", "work-log", "--since", "1h"})
	if err != nil {
		t.Fatalf("parseHistoryArgs returned error: %v", err)
	}
	if since == 0 {
		t.Fatal("expected non-zero since")
	}
	if !repoScope {
		t.Fatal("expected repo scope")
	}
	if !lineage {
		t.Fatal("expected lineage")
	}
	if format != "work-log" {
		t.Fatalf("format = %q", format)
	}
}

func TestParseHistoryArgsRejectsUnknown(t *testing.T) {
	if _, _, _, _, err := parseHistoryArgs([]string{"--bad"}); err == nil {
		t.Fatal("expected error")
	}
}
