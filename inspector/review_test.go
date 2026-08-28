package inspector

import (
	"testing"
	"time"
)

func TestReviewDecisionBuildsSeparateEventAndTraceReceipt(t *testing.T) {
	d := ReviewDecision{ReviewID: "change-1", Decision: "accepted", Actor: "nicholas", Scope: "project", At: time.UnixMilli(1700000000000).UTC()}
	e, err := d.Event()
	if err != nil || e.Kind != "review.accepted" || e.ReviewID != "change-1" {
		t.Fatalf("event=%#v err=%v", e, err)
	}
	r, err := d.TraceReceipt()
	if err != nil || r["operation"] != "lens.review.accepted" || r["outcome"] != "accepted" {
		t.Fatalf("receipt=%#v err=%v", r, err)
	}
}
