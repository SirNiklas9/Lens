package inspector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildBehaviorCaptureIsVolatileAndRefreshable(t *testing.T) {
	source := filepath.Join(t.TempDir(), "apply.go")
	if err := os.WriteFile(source, []byte("package demo"), 0600); err != nil {
		t.Fatal(err)
	}
	thread := Thread{Version: "atlas.thread.v1", Entry: ThreadSymbol{ID: "apply", Name: "Apply", File: "apply.go", Line: 1}}
	r, err := BuildBehaviorCapture(thread, "project", "behavior-apply", "apply", source, 1234)
	if err != nil || !r.Volatile || r.Record.Metadata["refresh.verifier"] != "filesystem" || r.Record.Metadata["refresh.digest"] == "" || len(r.EvidenceRefs) != 2 {
		t.Fatalf("capture=%#v err=%v", r, err)
	}
}
