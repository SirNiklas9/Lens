package inspector

import (
	"context"
	"encoding/json"
	"os"
)

// FileSource is an explicit development/local adapter. It makes no claim that
// a file is canonical; production hosts should provide a Lexicon API adapter.
type FileSource struct{ Path string }

func (f FileSource) Snapshot(context.Context) (Snapshot, error) {
	b, err := os.ReadFile(f.Path)
	if err != nil { return Snapshot{}, err }
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil { return Snapshot{}, err }
	if err := s.Validate(); err != nil { return Snapshot{}, err }
	return s, nil
}
