// lens-observe is an opt-in foreground Git observer. It emits normalized file
// events to an explicit JSONL stream and never writes to the observed project.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/SirNiklas9/inspector/inspector"
)

func main() {
	root := flag.String("root", ".", "Git working tree to observe")
	out := flag.String("out", "", "explicit Lens JSONL event stream")
	interval := flag.Duration("interval", 0, "foreground repeat interval; omitted means one observation")
	flag.Parse()
	if *out == "" {
		fatal("--out is required")
	}
	if err := inspector.EnsureEventStream(*out); err != nil {
		fatal(err.Error())
	}
	for {
		events, err := inspector.GitChanges(*root)
		if err != nil {
			fatal(err.Error())
		}
		for _, event := range events {
			if err := inspector.AppendEvent(*out, event); err != nil {
				fatal(err.Error())
			}
			fmt.Printf("%s %s\n", event.Kind, event.File)
		}
		if *interval <= 0 {
			return
		}
		time.Sleep(*interval)
	}
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "lens-observe:", message)
	os.Exit(2)
}
