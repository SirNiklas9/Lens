package inspector_test

// This is deliberately an opt-in, real-process release-acceptance test. It
// proves the portable boundaries rather than importing another application's
// Go packages: Lexicon seeds/captures state, Pith previews then explicitly
// applies a bounded patch, Scout proposes without mutation, Receipts survives
// an append/restart/list cycle, and Inspector projects their emitted JSON
// contracts. Every application state directory and every source fixture is
// created under t.TempDir; canonical data and user configuration are untouched.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPortableApplicationFlow(t *testing.T) {
	if os.Getenv("RUN_PORTABLE_APP_E2E") != "1" {
		t.Skip("set RUN_PORTABLE_APP_E2E=1 to run real Lexicon/Scout/Receipts/Inspector processes")
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	root := filepath.Dir(filepath.Dir(file)) // C:\dev
	tmp := t.TempDir()
	cache := filepath.Join(tmp, "go-cache")
	evidence := filepath.Join(tmp, "evidence.txt")
	if err := os.WriteFile(evidence, []byte("e2e evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256File(t, evidence)
	seed := filepath.Join(tmp, "lexicon.seed.toml")
	seedText := "format=1\nsource='portable-e2e'\n[[records]]\nscope='project'\nid='e2e-freshness'\nnamespace='e2e'\nkey='freshness'\nbody='e2e evidence contract'\nprovenance='portable-e2e'\n[records.metadata]\n'lifecycle.state'='candidate'\n'refresh.verifier'='filesystem'\n'refresh.digest'='" + digest + "'\n'refresh.path'='" + strings.ReplaceAll(evidence, "\\", "\\\\") + "'\n"
	if err := os.WriteFile(seed, []byte(seedText), 0600); err != nil {
		t.Fatal(err)
	}
	lexiconApp := filepath.Join(root, "lexicon", "application", "pulp.app.toml")
	lexiconState := filepath.Join(tmp, "lexicon-state")
	lexiconDir := filepath.Join(root, "lexicon", "cmd", "projects-knowledge")
	// The seed is explicitly planned first, then applied.
	runGo(t, cache, lexiconDir, "--app", lexiconApp, "--storage-root", lexiconState, "seed", seed)
	runGo(t, cache, lexiconDir, "--app", lexiconApp, "--storage-root", lexiconState, "seed", seed, "--apply")
	// Safe capture is likewise proposed then explicitly approved.
	capture := filepath.Join(tmp, "capture.json")
	if err := os.WriteFile(capture, []byte(`{"scope":"project","record":{"id":"e2e-capture","namespace":"e2e","key":"capture","body":"Y2FwdHVyZWQ=","metadata":{}},"provenance":"portable-e2e","evidence_refs":["e2e:fixture"],"volatile":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	runGo(t, cache, lexiconDir, "--app", lexiconApp, "--storage-root", lexiconState, "capture", capture)
	runGo(t, cache, lexiconDir, "--app", lexiconApp, "--storage-root", lexiconState, "capture", capture, "--apply")

	// Atlas -> Lens -> Lexicon is the explicit behavior-knowledge path. Atlas
	// supplies only structural evidence; Lens creates a reviewed capture packet;
	// Lexicon is the only owning store. After the declared source drifts, Scry
	// must propose a refresh rather than changing that record itself.
	behaviorSource := filepath.Join(tmp, "behavior.go")
	if err := os.WriteFile(behaviorSource, []byte("package fixture\n\nfunc watched() string { return \"before\" }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	atlasDir := filepath.Join(root, "atlas", "cmd", "atlas")
	atlasSnapshot := filepath.Join(tmp, "atlas.snapshot.json")
	threadFile := filepath.Join(tmp, "watched.thread.json")
	runGo(t, cache, atlasDir, "--root", tmp, "--output", atlasSnapshot, "snapshot")
	runGo(t, cache, atlasDir, "--snapshot", atlasSnapshot, "--symbol", "watched", "--output", threadFile, "thread")
	behaviorCapture := filepath.Join(tmp, "behavior-capture.json")
	behaviorCaptureDir := filepath.Join(root, "inspector", "cmd", "lens-behavior-capture")
	runGo(t, cache, behaviorCaptureDir,
		"--thread-file", threadFile, "--source", behaviorSource, "--scope", "project",
		"--id", "behavior-watched", "--key", "watched", "--review-at", "1000", "--output", behaviorCapture)
	runGo(t, cache, lexiconDir, "--app", lexiconApp, "--storage-root", lexiconState, "capture", behaviorCapture)
	runGo(t, cache, lexiconDir, "--app", lexiconApp, "--storage-root", lexiconState, "capture", behaviorCapture, "--apply")
	if err := os.WriteFile(behaviorSource, []byte("package fixture\n\nfunc watched() string { return \"after\" }\n"), 0600); err != nil {
		t.Fatal(err)
	}

	lexiconOutput := runGo(t, cache, lexiconDir, "--app", lexiconApp, "--storage-root", lexiconState, "inspect", "snapshot")
	lexiconJSON := writeJSONOutput(t, tmp, "lexicon.json", lexiconOutput)
	if !strings.Contains(lexiconOutput, "e2e-capture") || !strings.Contains(lexiconOutput, "e2e-freshness") {
		t.Fatalf("Lexicon snapshot did not contain explicitly applied seed/capture records: %s", lexiconOutput)
	}

	// Pith gets a deterministic local completion command. The preview must not
	// write the bounded source range; the second, explicit --apply may change
	// only that range and must leave a nearby sentinel untouched.
	pithDir := filepath.Join(root, "pith", "cmd", "pith")
	pithApp := filepath.Join(root, "pith", "application", "pulp.app.toml")
	pithSource := filepath.Join(tmp, "pith-target.go")
	pithSentinel := filepath.Join(tmp, "pith-sentinel.txt")
	pithOriginal := "package fixture\n\nfunc value() string {\n\treturn \"old\"\n}\n"
	if err := os.WriteFile(pithSource, []byte(pithOriginal), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pithSentinel, []byte("must not change\n"), 0600); err != nil {
		t.Fatal(err)
	}
	provider := writeFixedProvider(t, tmp, "\treturn \"new\"")
	preview := runGo(t, cache, pithDir, "edit", pithSource, "--range", "4:4", "--prompt", "replace value", "--cmd", provider, "--pith-app", pithApp, "--pith-runtime", filepath.Join(tmp, "pith-runtime"))
	if !strings.Contains(preview, "bounded proposal") {
		t.Fatalf("Pith preview did not identify its bounded contract: %s", preview)
	}
	assertFile(t, pithSource, pithOriginal, "Pith preview wrote source")
	assertFile(t, pithSentinel, "must not change\n", "Pith preview changed unrelated source")
	runGo(t, cache, pithDir, "edit", pithSource, "--range", "4:4", "--prompt", "replace value", "--cmd", provider, "--apply", "--pith-app", pithApp, "--pith-runtime", filepath.Join(tmp, "pith-runtime"))
	assertFile(t, pithSource, "package fixture\n\nfunc value() string {\n\treturn \"new\"\n}\n", "Pith explicit apply did not make exactly the bounded change")
	assertFile(t, pithSentinel, "must not change\n", "Pith apply changed unrelated source")

	scoutDir := filepath.Join(root, "scout", "cmd", "scout")
	scoutOutput := runGo(t, cache, scoutDir,
		"--app", filepath.Join(root, "scout", "application", "pulp.app.toml"), "--storage-root", filepath.Join(tmp, "scout-state"),
		"--lexicon-app", lexiconApp, "--lexicon-storage-root", lexiconState, "--allow-root", tmp, "review")
	scoutJSON := writeJSONOutput(t, tmp, "scout.json", scoutOutput)
	var scout map[string]any
	if err := json.Unmarshal([]byte(scoutOutput), &scout); err != nil {
		t.Fatal(err)
	}
	if n, _ := scout["proposals"].(float64); n < 1 {
		t.Fatalf("expected Scout proposal, got %s", scoutOutput)
	}
	behaviorScoutOutput := runGo(t, cache, scoutDir,
		"--app", filepath.Join(root, "scout", "application", "pulp.app.toml"), "--storage-root", filepath.Join(tmp, "behavior-scout-state"),
		"--lexicon-app", lexiconApp, "--lexicon-storage-root", lexiconState, "--allow-root", tmp, "--now", "1001", "review")
	if !strings.Contains(behaviorScoutOutput, "behavior-watched") || !strings.Contains(behaviorScoutOutput, "revise") {
		t.Fatalf("Scry did not emit a behavior refresh proposal after source drift: %s", behaviorScoutOutput)
	}
	// Scout is proposal-only. Its review is allowed to write its own temporary
	// report/state, but must never mutate Lexicon's canonical state.
	lexiconAfterScout := runGo(t, cache, lexiconDir, "--app", lexiconApp, "--storage-root", lexiconState, "inspect", "snapshot")
	if lexiconAfterScout != lexiconOutput {
		t.Fatalf("Scout review mutated Lexicon state\nbefore: %s\nafter: %s", lexiconOutput, lexiconAfterScout)
	}

	receipt := filepath.Join(tmp, "receipt.json")
	if err := os.WriteFile(receipt, []byte(`{"id":"e2e-receipt","operation":"scout.review","actor":"scout","scope":"project","outcome":"verified","output_digest":"sha256:e2e","evidence_refs":["e2e:scout"],"test_refs":["e2e:portable"],"occurred_at":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	receiptsDir := filepath.Join(root, "receipts", "cmd", "receipts")
	receiptsApp := filepath.Join(root, "receipts", "application", "pulp.app.toml")
	receiptsState := filepath.Join(tmp, "receipts-state")
	runGo(t, cache, receiptsDir, "--app", receiptsApp, "--storage-root", receiptsState, "append", receipt)
	receiptsOutput := runGo(t, cache, receiptsDir, "--app", receiptsApp, "--storage-root", receiptsState, "list")
	receiptsJSON := writeJSONOutput(t, tmp, "receipts.json", receiptsOutput)

	contextJSON := filepath.Join(tmp, "context.json")
	if err := os.WriteFile(contextJSON, []byte(`{"session_id":"portable-e2e","manifest_digest":"sha256:e2e","mode":"baseline","rule_count":1,"knowledge_count":2,"code_map_version":"external"}`), 0600); err != nil {
		t.Fatal(err)
	}
	inspectorDir := filepath.Join(root, "inspector", "cmd", "inspector")
	inspectorOutput := runGo(t, cache, inspectorDir, "--snapshot", "--lexicon-snapshot-file", lexiconJSON, "--scout-report-file", scoutJSON, "--receipts-list-file", receiptsJSON, "--context-manifest-file", contextJSON)
	var projection struct {
		Lexicon  any `json:"lexicon"`
		Scout    any `json:"scout"`
		Receipts any `json:"receipts"`
		Context  struct {
			Mode string `json:"mode"`
		} `json:"context"`
		Health struct {
			State string `json:"state"`
		} `json:"health"`
	}
	if err := json.Unmarshal([]byte(inspectorOutput), &projection); err != nil {
		t.Fatal(err)
	}
	if projection.Lexicon == nil || projection.Scout == nil || projection.Receipts == nil || projection.Context.Mode != "baseline" || projection.Health.State != "healthy" {
		t.Fatalf("Inspector did not project all applications: %s", inspectorOutput)
	}
	aiOutput := runGo(t, cache, inspectorDir, "--ai-context", "--lexicon-snapshot-file", lexiconJSON, "--scout-report-file", scoutJSON, "--receipts-list-file", receiptsJSON, "--context-manifest-file", contextJSON)
	var ai struct {
		Version string `json:"version"`
		Context struct {
			Mode string `json:"mode"`
		} `json:"context"`
		Scout struct {
			Proposals int `json:"proposals"`
		} `json:"scout"`
		Receipts struct {
			Count int `json:"count"`
		} `json:"receipts"`
	}
	if err := json.Unmarshal([]byte(aiOutput), &ai); err != nil {
		t.Fatal(err)
	}
	if ai.Version != "inspector.ai-context.v1" || ai.Context.Mode != "baseline" || ai.Scout.Proposals < 1 || ai.Receipts.Count != 1 {
		t.Fatalf("AI context did not summarize real apps: %s", aiOutput)
	}
	// The user-facing foreground entrypoint performs the same handoff without
	// relying on this test. It must create fresh artifacts and consume them.
	suiteOutput := runGo(t, cache, filepath.Join(root, "inspector", "cmd", "inspector-suite"),
		"--inspector-root", filepath.Join(root, "inspector"), "--lexicon-root", filepath.Join(root, "lexicon"), "--scout-root", filepath.Join(root, "scout"), "--receipts-root", filepath.Join(root, "receipts"),
		"--lexicon-storage-root", lexiconState, "--scout-storage-root", filepath.Join(tmp, "suite-scout-state"), "--receipts-storage-root", receiptsState,
		"--artifacts-dir", filepath.Join(tmp, "suite-artifacts"), "--context-manifest-file", contextJSON, "--allow-root", tmp, "collect")
	var suiteAI struct {
		Version string `json:"version"`
		Scout   struct {
			Proposals int `json:"proposals"`
		} `json:"scout"`
		Receipts struct {
			Count int `json:"count"`
		} `json:"receipts"`
	}
	if err := json.Unmarshal([]byte(suiteOutput), &suiteAI); err != nil {
		t.Fatal(err)
	}
	if suiteAI.Version != "inspector.ai-context.v1" || suiteAI.Scout.Proposals < 1 || suiteAI.Receipts.Count != 1 {
		t.Fatalf("suite output did not contain real app summaries: %s", suiteOutput)
	}
}

func writeFixedProvider(t *testing.T, dir, replacement string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "pith-replacement.cmd")
		if err := os.WriteFile(path, []byte("@echo off\r\necho "+replacement+"\r\n"), 0700); err != nil {
			t.Fatal(err)
		}
		return path
	}
	path := filepath.Join(dir, "pith-replacement.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '%s\\n' '"+replacement+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertFile(t *testing.T, path, want, message string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s\nwant: %q\n got: %q", message, want, string(got))
	}
}

func runGo(t *testing.T, cache, dir string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", append([]string{"run", "."}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE="+cache, "PULP_WAZERO_CACHE="+filepath.Join(cache, "wazero"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s: %v\nstderr: %s", strings.Join(cmd.Args, " "), err, string(exitStderr(err)))
	}
	return string(out)
}

func writeJSONOutput(t *testing.T, dir, name, raw string) string {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("non-JSON app output: %v\n%s", err, raw)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func exitStderr(err error) []byte {
	if e, ok := err.(*exec.ExitError); ok {
		return e.Stderr
	}
	return nil
}

func sha256File(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
