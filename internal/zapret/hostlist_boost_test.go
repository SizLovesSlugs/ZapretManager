package zapret

import (
	"strings"
	"testing"
)

func TestApplyBoostHostlistBlock(t *testing.T) {
	base := "# Never leave this file empty\ndomain.example.abc\n"
	on := applyBoostHostlistBlock(base, []string{"instagram.com", "www.facebook.com", "instagram.com"})
	if !strings.Contains(on, boostHostlistBegin) || !strings.Contains(on, "instagram.com\n") {
		t.Fatalf("expected boost block:\n%s", on)
	}
	if strings.Count(on, "instagram.com\n") != 1 {
		t.Fatalf("dedupe failed:\n%s", on)
	}
	if !strings.Contains(on, "domain.example.abc") {
		t.Fatal("lost placeholder")
	}
	off := applyBoostHostlistBlock(on, nil)
	if strings.Contains(off, boostHostlistBegin) {
		t.Fatalf("block should be removed:\n%s", off)
	}
	if !strings.Contains(off, "domain.example.abc") {
		t.Fatal("lost placeholder after clear")
	}
}
