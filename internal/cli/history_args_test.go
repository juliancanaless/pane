package cli

import "testing"

func TestParseHistoryArgsRepoAndSince(t *testing.T) {
	since, repoScope, lineage, err := parseHistoryArgs([]string{"--repo", "--lineage", "--since", "1h"})
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
}

func TestParseHistoryArgsRejectsUnknown(t *testing.T) {
	if _, _, _, err := parseHistoryArgs([]string{"--bad"}); err == nil {
		t.Fatal("expected error")
	}
}
