package inspector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEventAtlasSourceResolvesChangedFileToSymbols(t *testing.T) {
	dir := t.TempDir()
	events := filepath.Join(dir, "events.jsonl")
	atlas := filepath.Join(dir, "atlas.json")
	if err := os.WriteFile(events, []byte(`{"id":"f","kind":"file.modified","file":"internal/a.go"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(atlas, []byte(`{"version":"atlas.snapshot.v1","root":"C:/repo","symbols":[{"id":"a","name":"Apply","kind":"func","file":"internal/a.go","line":10},{"id":"b","name":"Verify","kind":"func","file":"internal/a.go","line":30}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := (EventAtlasSource{Events: EventFileSource{Path: events}, Atlas: AtlasFileSource{Path: atlas}}).Snapshot(context.Background())
	if err != nil || len(s.Live.ReviewQueue) != 2 || s.Live.ReviewQueue[0].Symbol != "Apply" || s.Live.ReviewQueue[1].Symbol != "Verify" {
		t.Fatalf("live=%#v err=%v", s.Live, err)
	}
}

func TestEventAtlasSourceUsesHunkOverlapNotWholeFile(t *testing.T) {
	dir := t.TempDir()
	events, atlas := filepath.Join(dir, "events.jsonl"), filepath.Join(dir, "atlas.json")
	if err := os.WriteFile(events, []byte(`{"id":"f","kind":"file.modified","file":"internal/a.go","start_line":20,"end_line":21}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(atlas, []byte(`{"version":"atlas.snapshot.v1","root":"C:/repo","symbols":[{"id":"a","name":"Apply","kind":"func","file":"internal/a.go","line":10},{"id":"b","name":"Verify","kind":"func","file":"internal/a.go","line":20},{"id":"c","name":"Later","kind":"func","file":"internal/a.go","line":40}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := (EventAtlasSource{Events: EventFileSource{Path: events}, Atlas: AtlasFileSource{Path: atlas}}).Snapshot(context.Background())
	if err != nil || len(s.Live.ReviewQueue) != 1 || s.Live.ReviewQueue[0].Symbol != "Verify" {
		t.Fatalf("live=%#v err=%v", s.Live, err)
	}
}
