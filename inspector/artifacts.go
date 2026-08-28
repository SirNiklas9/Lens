package inspector

// Portable artifact adapters ingest the explicit JSON contracts emitted by the
// other applications. They do not launch processes, discover files, or make
// any application authoritative over another: a caller supplies the paths it
// wants Inspector to project.
import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

type Artifacts struct {
	LexiconSnapshot string
	ScoutReport     string
	ReceiptsList    string
	ContextManifest string
}

type ArtifactSource struct{ Artifacts Artifacts }

func (a ArtifactSource) Snapshot(context.Context) (Snapshot, error) {
	out := Snapshot{Version: "inspector.snapshot.v1"}
	if a.Artifacts.LexiconSnapshot != "" {
		v, err := readArtifact(a.Artifacts.LexiconSnapshot)
		if err != nil {
			return Snapshot{}, fmt.Errorf("lexicon snapshot: %w", err)
		}
		out.Lexicon = v
	}
	if a.Artifacts.ScoutReport != "" {
		v, err := readArtifact(a.Artifacts.ScoutReport)
		if err != nil {
			return Snapshot{}, fmt.Errorf("scout report: %w", err)
		}
		out.Scout = v
	}
	if a.Artifacts.ReceiptsList != "" {
		v, err := readArtifact(a.Artifacts.ReceiptsList)
		if err != nil {
			return Snapshot{}, fmt.Errorf("receipts list: %w", err)
		}
		out.Receipts = v
	}
	if a.Artifacts.ContextManifest != "" {
		b, err := os.ReadFile(a.Artifacts.ContextManifest)
		if err != nil {
			return Snapshot{}, fmt.Errorf("context manifest: %w", err)
		}
		if err = json.Unmarshal(b, &out.Context); err != nil {
			return Snapshot{}, fmt.Errorf("context manifest: %w", err)
		}
	}
	if out.Lexicon == nil && out.Scout == nil && out.Receipts == nil && out.Context == (Context{}) {
		return Snapshot{}, fmt.Errorf("no Inspector artifacts declared")
	}
	return out, out.Validate()
}

func readArtifact(path string) (any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v any
	if err = json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return v, nil
}
