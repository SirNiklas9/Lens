package inspector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AtlasSnapshot is the small public portion of atlas.snapshot.v1 Lens needs to
// turn a changed file into source-backed changed symbols. It intentionally
// contains locations and identities, not source bodies.
type AtlasSnapshot struct {
	Version string        `json:"version"`
	Root    string        `json:"root"`
	Symbols []AtlasSymbol `json:"symbols"`
}

type AtlasSymbol struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type AtlasFileSource struct{ Path string }

func (f AtlasFileSource) Load() (AtlasSnapshot, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return AtlasSnapshot{}, err
	}
	var snapshot AtlasSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return AtlasSnapshot{}, err
	}
	if snapshot.Version != "atlas.snapshot.v1" {
		return AtlasSnapshot{}, fmt.Errorf("expected atlas.snapshot.v1 artifact")
	}
	return snapshot, nil
}

// EventAtlasSource enriches file-level events with the declarations Atlas
// actually maps in those files. If Atlas has no declaration, the original file
// review item remains—absence of a symbol is never fabricated into one.
type EventAtlasSource struct {
	Events EventFileSource
	Atlas  AtlasFileSource
}

func (s EventAtlasSource) Snapshot(ctx context.Context) (Snapshot, error) {
	base, err := s.Events.Snapshot(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	atlas, err := s.Atlas.Load()
	if err != nil {
		return Snapshot{}, err
	}
	if base.Live == nil {
		return base, nil
	}
	byFile := map[string][]AtlasSymbol{}
	for _, symbol := range atlas.Symbols {
		file := filepath.ToSlash(filepath.Clean(symbol.File))
		byFile[file] = append(byFile[file], symbol)
	}
	for file := range byFile {
		sort.Slice(byFile[file], func(i, j int) bool { return byFile[file][i].Line < byFile[file][j].Line })
	}
	var queue []ReviewItem
	for _, item := range base.Live.ReviewQueue {
		file := filepath.ToSlash(filepath.Clean(item.File))
		matches := byFile[file]
		if len(matches) == 0 {
			queue = append(queue, item)
			continue
		}
		for index, symbol := range matches {
			// A declaration's end is its next sibling declaration minus one.
			// Last declarations are conservatively open-ended. This gives hunk
			// overlap semantics without parsing source bodies in Lens.
			if item.StartLine > 0 && !overlapsDeclaration(item.StartLine, item.EndLine, symbol.Line, nextSymbolLine(matches, index)) {
				continue
			}
			resolved := item
			resolved.ID = item.ID + ":" + symbol.ID
			resolved.Symbol = symbol.Name
			resolved.StartLine = symbol.Line
			if resolved.Why == "" {
				resolved.Why = "resolved from Atlas structural evidence"
			}
			queue = append(queue, resolved)
		}
	}
	if len(queue) > 64 {
		queue = queue[len(queue)-64:]
	}
	base.Live.ReviewQueue = queue
	return base, nil
}

func nextSymbolLine(symbols []AtlasSymbol, index int) int {
	if index+1 >= len(symbols) {
		return int(^uint(0) >> 1)
	}
	return symbols[index+1].Line - 1
}

func overlapsDeclaration(changeStart, changeEnd, symbolStart, symbolEnd int) bool {
	if changeEnd == 0 {
		changeEnd = changeStart
	}
	return changeStart <= symbolEnd && changeEnd >= symbolStart
}

var _ Source = EventAtlasSource{}
var _ = strings.TrimSpace
