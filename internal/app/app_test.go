package app

import "testing"

func TestTargetVersion(t *testing.T) {
	if got := TargetVersion("latest", "1.10.1"); got != "1.10.1" {
		t.Fatalf("latest: %s", got)
	}
	if got := TargetVersion("", "1.10.1"); got != "1.10.1" {
		t.Fatalf("empty: %s", got)
	}
	if got := TargetVersion("1.9.9d", "1.10.1"); got != "1.9.9d" {
		t.Fatalf("pinned: %s", got)
	}
}

func TestFollowLatest(t *testing.T) {
	if !FollowLatest("latest") || !FollowLatest("") {
		t.Fatal("expected follow latest")
	}
	if FollowLatest("1.10.0") {
		t.Fatal("pinned should not follow")
	}
}
