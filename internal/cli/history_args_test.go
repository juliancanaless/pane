package cli

import "testing"

func TestParseHistoryArgsRepoAndSince(t *testing.T) {
	since, repoScope, err := parseHistoryArgs([]string{"--repo", "--since", "1h"})
	if err != nil {
		t.Fatalf("parseHistoryArgs returned error: %v", err)
	}
	if since == 0 {
		t.Fatal("expected non-zero since")
	}
	if !repoScope {
		t.Fatal("expected repo scope")
	}
}

func TestParseHistoryArgsRejectsUnknown(t *testing.T) {
	if _, _, err := parseHistoryArgs([]string{"--bad"}); err == nil {
		t.Fatal("expected error")
	}
}
