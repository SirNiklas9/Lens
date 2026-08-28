package inspector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestArtifactSourceProjectsIndependentApplicationOutputs(t *testing.T) {
	d := t.TempDir()
	write := func(name, text string) string {
		p := filepath.Join(d, name)
		if err := os.WriteFile(p, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	s := ArtifactSource{Artifacts: Artifacts{
		LexiconSnapshot: write("lexicon.json", `{"records":2,"freshness":"healthy"}`), ScoutReport: write("scout.json", `{"proposals":1,"skipped":0}`), ReceiptsList: write("receipts.json", `{"receipts":[{"id":"proof-1"}]}`), ContextManifest: write("context.json", `{"session_id":"s1","mode":"delta","rule_count":3,"knowledge_count":2,"code_map_version":"map-1"}`),
	}}
	out, err := s.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if out.Lexicon == nil || out.Scout == nil || out.Receipts == nil || out.Context.Mode != "delta" || out.Context.RuleCount != 3 {
		t.Fatalf("snapshot=%#v", out)
	}
}
