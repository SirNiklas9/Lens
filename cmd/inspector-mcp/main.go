// Command inspector-mcp exposes Inspector's existing read-only projections on
// MCP stdio. It owns no data and never starts a browser, service, or tool.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SirNiklas9/inspector/inspector"
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
type call struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}
type args struct {
	SnapshotFile        string `json:"snapshot_file"`
	LexiconSnapshotFile string `json:"lexicon_snapshot_file"`
	ScoutReportFile     string `json:"scout_report_file"`
	ReceiptsListFile    string `json:"receipts_list_file"`
	ContextManifestFile string `json:"context_manifest_file"`
	LiveEventsFile      string `json:"live_events_file"`
	AtlasSnapshotFile   string `json:"atlas_snapshot_file"`
	ThreadFile          string `json:"thread_file"`
}

func main() {
	if err := serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "inspector-mcp:", err)
		os.Exit(1)
	}
}

func serve(in io.Reader, out io.Writer) error {
	s := bufio.NewScanner(in)
	s.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	e := json.NewEncoder(out)
	for s.Scan() {
		var q request
		if json.Unmarshal(s.Bytes(), &q) != nil || len(q.ID) == 0 {
			continue
		}
		result, rpc := handle(q)
		if err := e.Encode(response{JSONRPC: "2.0", ID: q.ID, Result: result, Error: rpc}); err != nil {
			return err
		}
	}
	return s.Err()
}

func handle(q request) (any, *rpcError) {
	switch q.Method {
	case "initialize":
		return map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "lens", "version": "0.1.0"}, "instructions": "Lens is a read-only projection. It never owns or mutates Lexicon, Scry, Splice, or Trace state."}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": []map[string]any{
			tool("lens_snapshot", "Build one read-only Lens snapshot from explicit application artifacts."),
			tool("lens_ai_context", "Build a bounded no-source-body AI context from a Lens snapshot."),
			tool("lens_live_state", "Read the bounded live task, validation, and review projection from an explicit event stream."),
			tool("lens_changed_symbols", "Read changed symbols resolved only from an explicit Atlas snapshot."),
			tool("lens_thread", "Read an explicit Atlas Thread structural-path artifact."),
		}}, nil
	case "tools/call":
		var c call
		if json.Unmarshal(q.Params, &c) != nil {
			return nil, &rpcError{-32602, "invalid tools/call parameters"}
		}
		var a args
		if len(c.Arguments) > 0 && json.Unmarshal(c.Arguments, &a) != nil {
			return nil, &rpcError{-32602, "invalid tool arguments"}
		}
		var v any = inspector.Build(context.Background(), sources(a))
		if c.Name == "lens_ai_context" || c.Name == "inspector_ai_context" {
			var err error
			v, err = inspector.BuildAIContext(v.(inspector.Snapshot))
			if err != nil {
				return nil, &rpcError{-32603, err.Error()}
			}
		} else if c.Name == "lens_live_state" || c.Name == "lens_changed_symbols" || c.Name == "lens_thread" {
			v = v.(inspector.Snapshot)
		} else if c.Name != "lens_snapshot" && c.Name != "inspector_snapshot" {
			return nil, &rpcError{-32602, "unknown Inspector tool: " + c.Name}
		}
		b, err := json.Marshal(v)
		if err != nil {
			return nil, &rpcError{-32603, "encode Inspector result"}
		}
		return map[string]any{"content": []any{map[string]string{"type": "text", "text": string(b)}}}, nil
	default:
		return nil, &rpcError{-32601, "method not found: " + q.Method}
	}
}
func sources(a args) map[string]inspector.Source {
	out := map[string]inspector.Source{}
	if a.SnapshotFile != "" {
		out["lexicon"] = inspector.FileSource{Path: a.SnapshotFile}
	}
	artifacts := inspector.Artifacts{LexiconSnapshot: a.LexiconSnapshotFile, ScoutReport: a.ScoutReportFile, ReceiptsList: a.ReceiptsListFile, ContextManifest: a.ContextManifestFile}
	if artifacts.LexiconSnapshot != "" || artifacts.ScoutReport != "" || artifacts.ReceiptsList != "" || artifacts.ContextManifest != "" {
		out["applications"] = inspector.ArtifactSource{Artifacts: artifacts}
	}
	if a.LiveEventsFile != "" {
		if a.AtlasSnapshotFile != "" {
			out["live"] = inspector.EventAtlasSource{Events: inspector.EventFileSource{Path: a.LiveEventsFile}, Atlas: inspector.AtlasFileSource{Path: a.AtlasSnapshotFile}}
		} else {
			out["live"] = inspector.EventFileSource{Path: a.LiveEventsFile}
		}
	}
	if a.ThreadFile != "" {
		out["thread"] = mcpThreadSource{path: a.ThreadFile}
	}
	return out
}

type mcpThreadSource struct{ path string }

func (t mcpThreadSource) Snapshot(context.Context) (inspector.Snapshot, error) {
	return inspector.ThreadFileSource{Path: t.path}.Snapshot()
}
func tool(name, description string) map[string]any {
	return map[string]any{"name": name, "description": description, "inputSchema": map[string]any{"type": "object", "additionalProperties": true}}
}

// Keep flag imported by the command's module tooling without making the MCP
// protocol accept host-local listening configuration.
var _ = flag.CommandLine
var _ = strings.TrimSpace
