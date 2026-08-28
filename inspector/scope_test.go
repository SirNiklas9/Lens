package inspector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEventScopeSourceFlagsUnexpectedFileWithoutBlocking(t *testing.T) {
	dir := t.TempDir()
	events, policy := filepath.Join(dir, "events.jsonl"), filepath.Join(dir, "scope.json")
	if err := os.WriteFile(events, []byte(`{"id":"one","kind":"file.modified","file":"internal/links/a.go"}`+"\n"+`{"id":"two","kind":"file.modified","file":"deploy/a.go"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, []byte(`{"version":"lens.scope.v1","expected_files_max":1,"allowed_paths":["internal/links/**"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := (EventScopeSource{Base: EventFileSource{Path: events}, Policy: ScopeFileSource{Path: policy}}).Snapshot(context.Background())
	if err != nil || s.Live.Scope.State != "expanded" || len(s.Live.Scope.UnexpectedFiles) != 1 {
		t.Fatalf("scope=%#v err=%v", s.Live.Scope, err)
	}
}
