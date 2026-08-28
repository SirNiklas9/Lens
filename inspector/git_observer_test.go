package inspector

import "testing"

func TestParseNameStatusKeepsRenamedDestination(t *testing.T) {
	got := parseNameStatus("M\tmain.go\nA\tnew.go\nR100\told.go\trenamed.go\n")
	if got["main.go"] != "M" || got["new.go"] != "A" || got["renamed.go"] != "R" {
		t.Fatalf("states=%#v", got)
	}
}

func TestParseUnifiedZeroReadsNewFileHunkRanges(t *testing.T) {
	got := parseUnifiedZero("diff --git a/a.go b/a.go\n+++ b/a.go\n@@ -10,2 +10,3 @@\n@@ -30 +31,0 @@\n")
	ranges := got["a.go"]
	if len(ranges) != 2 || ranges[0] != (lineRange{10, 12}) || ranges[1] != (lineRange{31, 31}) {
		t.Fatalf("ranges=%#v", ranges)
	}
}
