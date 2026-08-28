package inspector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestThreadFileSourceReadsAtlasArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thread.json")
	data := `{"version":"atlas.thread.v1","root":"C:/repo","entry":{"id":"a","name":"Apply","kind":"func","file":"a.go","line":1},"nodes":[{"symbol":{"id":"a","name":"Apply","kind":"func","file":"a.go","line":1}}]}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := (ThreadFileSource{Path: path}).Snapshot()
	if err != nil || len(s.Threads) != 1 || s.Threads[0].Entry.Name != "Apply" {
		t.Fatalf("snapshot=%#v err=%v", s, err)
	}
}
