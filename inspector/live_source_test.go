package inspector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestEventFileSourceBuildsBoundedReviewProjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	content := "{\"id\":\"goal\",\"kind\":\"task.updated\",\"task_id\":\"task-1\",\"why\":\"Fix link ordering\"}\n" +
		"{\"id\":\"start\",\"kind\":\"edit.started\",\"file\":\"internal/links/apply.go\",\"symbol\":\"prepareDestination\",\"why\":\"validate source first\"}\n" +
		"{\"id\":\"change\",\"kind\":\"symbol.touched\",\"file\":\"internal/links/apply.go\",\"start_line\":71,\"end_line\":89,\"symbol\":\"prepareDestination\",\"why\":\"validation before removal\"}\n" +
		"{\"id\":\"test\",\"kind\":\"validation.passed\",\"file\":\"internal/links/apply.go\",\"symbol\":\"go test ./internal/links\"}\n" +
		"{\"id\":\"accepted\",\"kind\":\"review.accepted\",\"review_id\":\"change\"}\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := (EventFileSource{Path: path}).Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.Live == nil || s.Live.Goal != "Fix link ordering" || s.Live.Current == nil {
		t.Fatalf("live=%#v", s.Live)
	}
	if len(s.Live.ReviewQueue) != 1 || s.Live.ReviewQueue[0].State != "ACCEPTED" {
		t.Fatalf("queue=%#v", s.Live.ReviewQueue)
	}
	if len(s.Live.Validation) != 1 || s.Live.Validation[0].State != "PASSED" {
		t.Fatalf("validation=%#v", s.Live.Validation)
	}
}

func TestEventFileSourceRejectsAmbiguousEvent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte("{\"kind\":\"file.modified\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := (EventFileSource{Path: path}).Snapshot(context.Background()); err == nil {
		t.Fatal("expected invalid event rejection")
	}
}

func TestEventFileSourceAcceptsPowerShellUTF8BOM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"id":"one","kind":"task.started"}`+"\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := (EventFileSource{Path: path}).Snapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureEventStreamCreatesReadableEmptyStream(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events", "live.jsonl")
	if err := EnsureEventStream(path); err != nil {
		t.Fatal(err)
	}
	s, err := (EventFileSource{Path: path}).Snapshot(context.Background())
	if err != nil || s.Live == nil || len(s.Live.Recent) != 0 {
		t.Fatalf("snapshot=%#v err=%v", s, err)
	}
}

func TestAppendEventSeparatesPriorJSONWithoutFinalNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte(`{"id":"one","kind":"task.started"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := AppendEvent(path, Event{ID: "two", Kind: "task.completed"}); err != nil {
		t.Fatal(err)
	}
	s, err := (EventFileSource{Path: path}).Snapshot(context.Background())
	if err != nil || len(s.Live.Recent) != 2 {
		t.Fatalf("live=%#v err=%v", s.Live, err)
	}
}

func TestEventFileSourceBoundsReviewQueue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	var content string
	for i := 0; i < 65; i++ {
		content += fmt.Sprintf(`{"id":"change-%d","kind":"symbol.touched","file":"a.go"}`+"\n", i)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := (EventFileSource{Path: path}).Snapshot(context.Background())
	if err != nil || len(s.Live.ReviewQueue) != 64 || s.Live.ReviewQueue[0].ID != "change-1" {
		t.Fatalf("queue=%#v err=%v", s.Live.ReviewQueue, err)
	}
}
