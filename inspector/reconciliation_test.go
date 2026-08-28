package inspector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestIntentReconciliationFlagsEvidenceMismatch(t *testing.T) {
	dir := t.TempDir()
	events, intent := filepath.Join(dir, "events.jsonl"), filepath.Join(dir, "intent.json")
	content := `{"id":"one","kind":"symbol.touched","file":"internal/a.go","symbol":"Apply"}` + "\n" + `{"id":"two","kind":"validation.failed","file":"internal/a.go","symbol":"test"}` + "\n"
	if err := os.WriteFile(events, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(intent, []byte(`{"version":"lens.intent.v1","goal":"fix links","expected_paths":["internal/**"],"expected_symbols":["Apply","Verify"],"required_validations":["test"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := (EventIntentSource{Base: EventFileSource{Path: events}, Intent: IntentFileSource{Path: intent}}).Snapshot(context.Background())
	if err != nil || s.Reconciliation.State != "review_required" || len(s.Reconciliation.MissingSymbols) != 1 || len(s.Reconciliation.FailedValidations) != 1 {
		t.Fatalf("reconciliation=%#v err=%v", s.Reconciliation, err)
	}
}
