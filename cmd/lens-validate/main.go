// lens-validate runs one explicitly declared validation command and records
// normalized started/passed/failed events. It never guesses project commands,
// invokes a shell, or starts a background validation service.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/SirNiklas9/inspector/inspector"
)

func main() {
	events := flag.String("events", "", "explicit Lens event JSONL")
	id := flag.String("id", "", "validation identity")
	target := flag.String("target", "", "affected file or subsystem")
	kind := flag.String("kind", "command", "validation kind, e.g. parse|format|test")
	workdir := flag.String("cwd", "", "optional command working directory")
	actor := flag.String("actor", "validator", "receipt actor when Trace is configured")
	scope := flag.String("scope", "project", "receipt scope when Trace is configured")
	trace := flag.String("trace", "", "optional Trace executable for immutable validation receipt")
	traceApp := flag.String("trace-app", "", "Trace pulp.app.toml when --trace is used")
	traceStorage := flag.String("trace-storage-root", "", "Trace storage root when --trace is used")
	traceInstance := flag.String("trace-instance", "default", "Trace receipt instance")
	flag.Parse()
	command := flag.Args()
	if *events == "" || *id == "" || len(command) == 0 {
		fatal("--events, --id, and command arguments are required")
	}
	started := inspector.Event{ID: *id + ":started", Kind: "validation.started", File: *target, Symbol: *kind, Why: "explicit declared validation", At: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := inspector.AppendEvent(*events, started); err != nil {
		fatal(err.Error())
	}
	run := exec.Command(command[0], command[1:]...)
	run.Dir = *workdir
	run.Stdout, run.Stderr = os.Stdout, os.Stderr
	err := run.Run()
	kindEvent, outcome := "validation.passed", "passed"
	if err != nil {
		kindEvent, outcome = "validation.failed", "failed"
	}
	if *trace != "" {
		if *traceApp == "" || *traceStorage == "" { fatal("--trace-app and --trace-storage-root are required with --trace") }
		if traceErr := appendTrace(*trace, *traceApp, *traceStorage, *traceInstance, *id, *kind, *actor, *scope, outcome); traceErr != nil { fatal(traceErr.Error()) }
	}
	finished := inspector.Event{ID: *id + ":" + outcome, Kind: kindEvent, File: *target, Symbol: *kind, Why: "explicit declared validation " + outcome, Outcome: outcome, At: time.Now().UTC().Format(time.RFC3339Nano)}
	if appendErr := inspector.AppendEvent(*events, finished); appendErr != nil {
		fatal(appendErr.Error())
	}
	if err != nil {
		os.Exit(1)
	}
}

func appendTrace(binary, app, storage, instance, id, kind, actor, scope, outcome string) error {
	dir, err := os.MkdirTemp("", "lens-validation-")
	if err != nil { return err }
	defer os.RemoveAll(dir)
	receipt := map[string]any{
		"id": "lens-validation:" + id + ":" + outcome,
		"operation": "lens.validation." + kind,
		"actor": actor, "scope": scope, "outcome": outcome,
		"evidence_refs": []string{"lens:validation:" + id},
		"test_refs": []string{"lens:test:" + kind},
		"occurred_at": time.Now().UTC().UnixMilli(),
	}
	data, err := json.Marshal(receipt)
	if err != nil { return err }
	path := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(path, data, 0600); err != nil { return err }
	command := exec.Command(binary, "--app", app, "--storage-root", storage, "--instance", instance, "append", path)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	return command.Run()
}

func fatal(message string) { fmt.Fprintln(os.Stderr, "lens-validate:", message); os.Exit(2) }
