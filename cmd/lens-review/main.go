// lens-review persists one explicit human review action. When Trace paths are
// supplied, it appends the immutable receipt first; the Lens event is written
// only after Trace accepted the evidence.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/SirNiklas9/inspector/inspector"
)

func main() {
	events := flag.String("events", "", "explicit Lens event JSONL")
	reviewID := flag.String("review-id", "", "Lens review item identity")
	decision := flag.String("decision", "", "reviewed|accepted|rejected")
	actor := flag.String("actor", "human", "reviewing actor")
	scope := flag.String("scope", "project", "review scope")
	trace := flag.String("trace", "", "optional Trace executable")
	traceApp := flag.String("trace-app", "", "Trace pulp.app.toml when --trace is used")
	traceStorage := flag.String("trace-storage-root", "", "Trace storage root when --trace is used")
	traceInstance := flag.String("trace-instance", "default", "Trace receipt instance")
	flag.Parse()
	d := inspector.ReviewDecision{ReviewID: *reviewID, Decision: *decision, Actor: *actor, Scope: *scope}
	event, err := d.Event()
	if err != nil {
		fatal(err)
	}
	if *trace != "" {
		if *traceApp == "" || *traceStorage == "" {
			fatal(fmt.Errorf("--trace-app and --trace-storage-root are required with --trace"))
		}
		if err := appendTrace(*trace, *traceApp, *traceStorage, *traceInstance, d); err != nil {
			fatal(err)
		}
	}
	if err := inspector.AppendEvent(*events, event); err != nil {
		fatal(err)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"event_id": event.ID, "review_id": event.ReviewID, "decision": *decision, "trace_written": *trace != ""})
}

func appendTrace(binary, app, storage, instance string, decision inspector.ReviewDecision) error {
	receipt, err := decision.TraceReceipt()
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "lens-review-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "receipt.json")
	data, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	command := exec.Command(binary, "--app", app, "--storage-root", storage, "--instance", instance, "append", path)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	return command.Run()
}

func fatal(err error) { fmt.Fprintln(os.Stderr, "lens-review:", err); os.Exit(2) }
