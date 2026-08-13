package version

import "testing"

func TestIsOlder(t *testing.T) {
	cases := []struct {
		candidate string
		reference string
		want      bool
	}{
		{"0.1.4", "0.1.5", true},
		{"0.1.5", "0.1.5", false},
		{"0.1.6", "0.1.5", false},
		{"0.2.0", "0.1.5", false},
		{"1.0.0", "0.9.9", false},
		{"0.1", "0.1.5", true},
		{"v0.1.4", "0.1.5", true},
		{"", "0.1.5", true},
		{"garbage", "0.1.5", true},
		{"0.1.5", "garbage", false},
	}
	for _, c := range cases {
		if got := IsOlder(c.candidate, c.reference); got != c.want {
			t.Errorf("IsOlder(%q, %q) = %v, want %v", c.candidate, c.reference, got, c.want)
		}
	}
}
