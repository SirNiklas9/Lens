package inspector

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// ReviewDecision is an explicit human decision. It is intentionally separate
// from validation status: a passing check is evidence, not acceptance.
type ReviewDecision struct {
	ReviewID string
	Decision string // reviewed, accepted, rejected
	Actor    string
	Scope    string
	At       time.Time
}

func (d ReviewDecision) Validate() error {
	if strings.TrimSpace(d.ReviewID) == "" || strings.TrimSpace(d.Actor) == "" || strings.TrimSpace(d.Scope) == "" {
		return fmt.Errorf("review_id, actor, and scope are required")
	}
	if d.Decision != "reviewed" && d.Decision != "accepted" && d.Decision != "rejected" {
		return fmt.Errorf("decision must be reviewed, accepted, or rejected")
	}
	return nil
}

func (d ReviewDecision) eventID() string {
	at := d.At
	if at.IsZero() {
		at = time.Now().UTC()
	}
	sum := sha256.Sum256([]byte(d.ReviewID + "\x00" + d.Decision + "\x00" + d.Actor + "\x00" + at.Format(time.RFC3339Nano)))
	return "lens-review-" + hex.EncodeToString(sum[:12])
}

func (d ReviewDecision) Event() (Event, error) {
	if err := d.Validate(); err != nil {
		return Event{}, err
	}
	at := d.At
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return Event{ID: d.eventID(), Kind: "review." + d.Decision, ReviewID: d.ReviewID, At: at.Format(time.RFC3339Nano), Why: "explicit human review decision"}, nil
}

// TraceReceipt is compatible with Trace's standalone JSON append contract. It
// carries opaque identity/evidence only—no source, prompt, diff, or reasoning.
func (d ReviewDecision) TraceReceipt() (map[string]any, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	at := d.At
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return map[string]any{
		"id": d.eventID(), "operation": "lens.review." + d.Decision,
		"actor": d.Actor, "scope": d.Scope, "outcome": d.Decision,
		"evidence_refs": []string{"lens:review:" + d.ReviewID}, "occurred_at": at.UnixMilli(),
	}, nil
}
