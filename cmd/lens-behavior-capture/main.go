// lens-behavior-capture produces an explicit Lexicon capture proposal from an
// approved Atlas Thread. It writes no Lexicon state; use Lexicon's capture
// command to inspect, then explicitly apply the proposal.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/SirNiklas9/inspector/inspector"
)

func main() {
	threadPath := flag.String("thread-file", "", "atlas.thread.v1 artifact")
	source := flag.String("source", "", "declared source file to verify for freshness")
	scope := flag.String("scope", "project", "Lexicon target scope")
	id := flag.String("id", "", "Lexicon behavior record ID")
	key := flag.String("key", "", "Lexicon behavior key")
	reviewAt := flag.Int64("review-at", 0, "next source review Unix milliseconds")
	output := flag.String("output", "", "proposal JSON path; stdout when omitted")
	flag.Parse()
	data, err := os.ReadFile(*threadPath)
	if err != nil {
		fatal(err)
	}
	var thread inspector.Thread
	if err := json.Unmarshal(data, &thread); err != nil {
		fatal(err)
	}
	proposal, err := inspector.BuildBehaviorCapture(thread, *scope, *id, *key, *source, *reviewAt)
	if err != nil {
		fatal(err)
	}
	if *output == "" {
		_ = json.NewEncoder(os.Stdout).Encode(proposal)
		return
	}
	file, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		fatal(err)
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(proposal); err != nil {
		fatal(err)
	}
}
func fatal(err error) { fmt.Fprintln(os.Stderr, "lens-behavior-capture:", err); os.Exit(2) }
