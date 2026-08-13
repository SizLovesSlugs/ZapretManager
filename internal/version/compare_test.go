package version

import "testing"

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1", "0.1", 0},
		{"0.1", "0.1.0", 0},
		{"v0.2", "0.1", 1},
		{"0.1", "0.2", -1},
		{"0.1.9", "0.2", -1},
		{"1.0", "0.9", 1},
		{"1.0 Beta", "1.0-Beta", 0},
		{"", "0.1", -1},
	}
	for _, tc := range cases {
		if got := Compare(tc.a, tc.b); got != tc.want {
			t.Fatalf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestExeNameFor(t *testing.T) {
	if got := ExeNameFor("1.0"); got != "ZapretManager-1.0.exe" {
		t.Fatalf("tag: %q", got)
	}
	if got := ExeNameFor("v1.0"); got != "ZapretManager-1.0.exe" {
		t.Fatalf("v-prefix: %q", got)
	}
	if got := ExeName(); got != "ZapretManager-1.0.exe" {
		t.Fatalf("current: %q", got)
	}
	if got := ExeNameFor("1.1"); got != "ZapretManager-1.1.exe" {
		t.Fatalf("1.1: %q", got)
	}
}
