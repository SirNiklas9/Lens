package inspector

import (
	"encoding/json"
	"fmt"
	"os"
)

// Thread mirrors Atlas's public atlas.thread.v1 artifact without taking an
// Atlas implementation dependency. Lens renders this source-backed method map
// but does not own or regenerate structural evidence.
type Thread struct {
	Version   string       `json:"version"`
	Root      string       `json:"root"`
	Entry     ThreadSymbol `json:"entry"`
	Nodes     []ThreadNode `json:"nodes"`
	Truncated bool         `json:"truncated,omitempty"`
}

type ThreadSymbol struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type ThreadNode struct {
	Symbol  ThreadSymbol `json:"symbol"`
	Calls   []string     `json:"calls,omitempty"`
	Callers []string     `json:"callers,omitempty"`
}

// ThreadFileSource reads one caller-supplied Atlas Thread artifact. It accepts
// a bounded structural map only; source code and prose explanations remain in
// their own providers.
type ThreadFileSource struct{ Path string }

func (f ThreadFileSource) Snapshot() (Snapshot, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return Snapshot{}, err
	}
	var thread Thread
	if err := json.Unmarshal(data, &thread); err != nil {
		return Snapshot{}, err
	}
	if thread.Version != "atlas.thread.v1" || thread.Entry.ID == "" {
		return Snapshot{}, fmt.Errorf("expected atlas.thread.v1 artifact")
	}
	return Snapshot{Version: "inspector.snapshot.v1", Threads: []Thread{thread}}, nil
}
