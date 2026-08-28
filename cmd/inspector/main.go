// Command lens serves the read-only local Lens API or prints one
// snapshot. It is intentionally not a browser dashboard or harness plugin.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/SirNiklas9/inspector/inspector"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:7341", "local Lens API address")
	snapshot := flag.String("snapshot-file", "", "explicit read-only snapshot adapter JSON file")
	lexiconSnapshot := flag.String("lexicon-snapshot-file", "", "Lexicon inspect snapshot JSON emitted by its portable CLI")
	scoutReport := flag.String("scout-report-file", "", "Scout review report JSON emitted by its portable CLI")
	receiptsList := flag.String("receipts-list-file", "", "Receipts list JSON emitted by its portable CLI")
	contextManifest := flag.String("context-manifest-file", "", "host context manifest JSON")
	liveEvents := flag.String("live-events-file", "", "explicit newline-delimited provider-neutral Lens event stream")
	atlasSnapshot := flag.String("atlas-snapshot-file", "", "optional atlas.snapshot.v1 used to resolve live changed files into symbols")
	scopePolicy := flag.String("scope-policy-file", "", "optional lens.scope.v1 visibility budget for live changes")
	intentFile := flag.String("intent-file", "", "optional lens.intent.v1 reconciliation contract for live changes")
	threadFile := flag.String("thread-file", "", "explicit Atlas atlas.thread.v1 artifact")
	printOnly := flag.Bool("snapshot", false, "print snapshot JSON and exit")
	aiContext := flag.Bool("ai-context", false, "print bounded inspector.ai-context.v1 JSON and exit")
	flag.Parse()
	sources := map[string]inspector.Source{}
	if *snapshot != "" {
		sources["lexicon"] = inspector.FileSource{Path: *snapshot}
	}
	artifacts := inspector.Artifacts{LexiconSnapshot: *lexiconSnapshot, ScoutReport: *scoutReport, ReceiptsList: *receiptsList, ContextManifest: *contextManifest}
	if artifacts.LexiconSnapshot != "" || artifacts.ScoutReport != "" || artifacts.ReceiptsList != "" || artifacts.ContextManifest != "" {
		sources["applications"] = inspector.ArtifactSource{Artifacts: artifacts}
	}
	if *liveEvents != "" {
		var live inspector.Source
		if *atlasSnapshot != "" {
			live = inspector.EventAtlasSource{Events: inspector.EventFileSource{Path: *liveEvents}, Atlas: inspector.AtlasFileSource{Path: *atlasSnapshot}}
		} else {
			live = inspector.EventFileSource{Path: *liveEvents}
		}
		if *scopePolicy != "" {
			live = inspector.EventScopeSource{Base: live, Policy: inspector.ScopeFileSource{Path: *scopePolicy}}
		}
		if *intentFile != "" {
			live = inspector.EventIntentSource{Base: live, Intent: inspector.IntentFileSource{Path: *intentFile}}
		}
		sources["live"] = live
	}
	if *threadFile != "" {
		sources["thread"] = threadSource{path: *threadFile}
	}
	if *printOnly {
		if err := json.NewEncoder(os.Stdout).Encode(inspector.Build(context.Background(), sources)); err != nil {
			panic(err)
		}
		return
	}
	if *aiContext {
		value, err := inspector.BuildAIContext(inspector.Build(context.Background(), sources))
		if err != nil {
			panic(err)
		}
		if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
			panic(err)
		}
		return
	}
	fmt.Fprintf(os.Stderr, "Lens local API listening on http://%s/v1/snapshot\n", *listen)
	if err := http.ListenAndServe(*listen, inspector.Handler(sources)); err != nil {
		panic(err)
	}
}

type threadSource struct{ path string }

func (t threadSource) Snapshot(context.Context) (inspector.Snapshot, error) {
	return inspector.ThreadFileSource{Path: t.path}.Snapshot()
}
