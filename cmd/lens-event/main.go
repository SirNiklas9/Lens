// lens-event is the small provider-neutral publishing side of the Lens event
// adapter. It performs one explicit append to a caller-selected JSONL file and
// exits; it never watches a repository or launches a daemon.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/SirNiklas9/inspector/inspector"
)

func main() {
	var output string
	var event inspector.Event
	flag.StringVar(&output, "out", "", "explicit JSONL event stream to append")
	flag.StringVar(&event.ID, "id", "", "event identity")
	flag.StringVar(&event.Kind, "kind", "", "normalized event kind")
	flag.StringVar(&event.TaskID, "task", "", "optional task identity")
	flag.StringVar(&event.File, "file", "", "optional file path")
	flag.IntVar(&event.StartLine, "start-line", 0, "optional one-based start line")
	flag.IntVar(&event.EndLine, "end-line", 0, "optional one-based end line")
	flag.StringVar(&event.Symbol, "symbol", "", "optional symbol identity")
	flag.StringVar(&event.Why, "why", "", "optional explicit rationale")
	flag.StringVar(&event.Outcome, "outcome", "", "optional outcome")
	flag.StringVar(&event.ReviewID, "review-id", "", "original review item for reviewed/accepted/rejected transitions")
	flag.Parse()
	if output == "" {
		fatal("--out is required")
	}
	if err := inspector.ValidateEvent(event); err != nil {
		fatal(err.Error())
	}
	if err := inspector.AppendEvent(output, event); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("%s %s\n", event.ID, event.Kind)
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "lens-event:", message)
	os.Exit(2)
}
