package inspector

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAIContextIsBoundedAndExcludesBodies(t *testing.T) {
	s := Snapshot{Version: "inspector.snapshot.v1", Context: Context{Mode: "delta", RuleCount: 2, KnowledgeCount: 1}, Health: Health{State: "healthy"}, Lexicon: map[string]any{"records": []any{map[string]any{"id": "fact-1", "body": "must not appear"}}, "rules": []any{map[string]any{"body": "rule body"}}, "review": []any{map[string]any{"record": map[string]any{"id": "fact-1", "namespace": "architecture", "key": "storage", "body": "secret body"}, "decision": map[string]any{"state": "candidate"}}}}, Scout: map[string]any{"mode": "proposal-only", "candidates": 1, "proposals": 1, "items": []any{map[string]any{"record_id": "fact-1", "scope": "project", "proposal": map[string]any{"outcome": "revise", "body": "nope"}}}}, Receipts: map[string]any{"receipts": []any{map[string]any{"id": "r1", "operation": "pith.patch.propose", "outcome": "verified", "prompt": "nope"}}}}
	out, err := BuildAIContext(s)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if out.Version != aiContextVersion || out.Lexicon.RecordCount != 1 || out.Scout.Proposals != 1 || out.Receipts.Count != 1 {
		t.Fatalf("context=%s", got)
	}
	for _, forbidden := range []string{"must not appear", "rule body", "secret body", "nope", "prompt", "body"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("AI context leaked %q: %s", forbidden, got)
		}
	}
}
