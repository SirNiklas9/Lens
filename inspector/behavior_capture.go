package inspector

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BehaviorCaptureRequest is wire-compatible with Lexicon's explicit capture
// proposal input. Lens produces it but cannot apply it; Lexicon still validates
// and requires the operator's --apply approval.
type BehaviorCaptureRequest struct {
	Scope        string                `json:"scope"`
	Record       BehaviorCaptureRecord `json:"record"`
	Provenance   string                `json:"provenance"`
	EvidenceRefs []string              `json:"evidence_refs"`
	Volatile     bool                  `json:"volatile"`
	ReviewAt     int64                 `json:"review_at"`
}
type BehaviorCaptureRecord struct {
	ID        string            `json:"id"`
	Namespace string            `json:"namespace"`
	Key       string            `json:"key"`
	Body      []byte            `json:"body"`
	Metadata  map[string]string `json:"metadata"`
}

// BuildBehaviorCapture turns a bounded source-backed Thread into a volatile
// Lexicon proposal. The source is a specific declared file: Scry can later
// verify its digest and propose a refresh, but no component silently rewrites
// the approved knowledge record.
func BuildBehaviorCapture(thread Thread, scope, id, key, source string, reviewAt int64) (BehaviorCaptureRequest, error) {
	if thread.Version != "atlas.thread.v1" || thread.Entry.ID == "" {
		return BehaviorCaptureRequest{}, fmt.Errorf("expected atlas.thread.v1 with an entry")
	}
	if scope != "global" && scope != "workspace" && scope != "project" {
		return BehaviorCaptureRequest{}, fmt.Errorf("scope must be global, workspace, or project")
	}
	if strings.TrimSpace(id) == "" || strings.TrimSpace(key) == "" || strings.TrimSpace(source) == "" || reviewAt <= 0 {
		return BehaviorCaptureRequest{}, fmt.Errorf("id, key, source, and positive review_at are required")
	}
	abs, err := filepath.Abs(source)
	if err != nil {
		return BehaviorCaptureRequest{}, err
	}
	body, err := os.ReadFile(abs)
	if err != nil {
		return BehaviorCaptureRequest{}, err
	}
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	threadBytes, err := json.Marshal(thread)
	if err != nil {
		return BehaviorCaptureRequest{}, err
	}
	evidence := "file:" + filepath.ToSlash(abs) + "#sha256=" + digest
	return BehaviorCaptureRequest{
		Scope: scope, Provenance: "lens.thread.v1", EvidenceRefs: []string{evidence, "atlas:thread:" + thread.Entry.ID}, Volatile: true, ReviewAt: reviewAt,
		Record: BehaviorCaptureRecord{ID: id, Namespace: "behavior", Key: key, Body: threadBytes, Metadata: map[string]string{
			"lifecycle.state": "active", "refresh.verifier": "filesystem", "refresh.path": abs, "refresh.digest": digest,
			"lens.thread.version": thread.Version, "lens.thread.entry": thread.Entry.ID, "lens.thread.source": filepath.ToSlash(abs),
		}},
	}, nil
}
