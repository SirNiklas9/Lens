// Command inspector-suite is a portable foreground handoff runner. It calls
// the independent Lexicon, Scout, Receipts, and Inspector source applications,
// writes their explicit JSON artifacts to a caller-owned directory, then emits
// either inspector.snapshot.v1 or inspector.ai-context.v1. It installs
// nothing, starts no background service, and never writes Lexicon state.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if err := execute(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "inspector-suite:", err)
		os.Exit(1)
	}
}

func execute(args []string) error {
	flags := flag.NewFlagSet("inspector-suite", flag.ContinueOnError)
	inspectorRoot := flags.String("inspector-root", "", "Inspector repository root (defaults to current directory)")
	lexiconRoot := flags.String("lexicon-root", "", "Lexicon repository root")
	scoutRoot := flags.String("scout-root", "", "Scout repository root")
	receiptsRoot := flags.String("receipts-root", "", "Receipts repository root")
	lexiconState := flags.String("lexicon-storage-root", "", "Lexicon application storage root")
	scoutState := flags.String("scout-storage-root", "", "Scout application storage root")
	receiptsState := flags.String("receipts-storage-root", "", "Receipts application storage root")
	artifacts := flags.String("artifacts-dir", "", "new directory for explicit JSON handoffs")
	contextManifest := flags.String("context-manifest-file", "", "optional host context manifest JSON")
	mode := flags.String("mode", "ai-context", "Inspector output: ai-context or snapshot")
	var roots listFlag
	flags.Var(&roots, "allow-root", "Scout local evidence allow root (repeatable)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || flags.Arg(0) != "collect" {
		return errors.New("collect is required")
	}
	if *lexiconRoot == "" || *scoutRoot == "" || *receiptsRoot == "" || *lexiconState == "" || *scoutState == "" || *receiptsState == "" || *artifacts == "" {
		return errors.New("--lexicon-root, --scout-root, --receipts-root, all three --*-storage-root flags, and --artifacts-dir are required")
	}
	if *mode != "ai-context" && *mode != "snapshot" {
		return errors.New("--mode must be ai-context or snapshot")
	}
	if *inspectorRoot == "" {
		var err error
		*inspectorRoot, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(*artifacts, 0755); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	lexiconApp := filepath.Join(*lexiconRoot, "application", "pulp.app.toml")
	lexicon, err := runGo(ctx, filepath.Join(*lexiconRoot, "cmd", "projects-knowledge"), "--app", lexiconApp, "--storage-root", *lexiconState, "inspect", "snapshot")
	if err != nil {
		return fmt.Errorf("lexicon inspect: %w", err)
	}
	lexiconFile := filepath.Join(*artifacts, "lexicon-snapshot.json")
	if err = writeNew(lexiconFile, lexicon); err != nil {
		return err
	}
	scoutArgs := []string{"--app", filepath.Join(*scoutRoot, "application", "pulp.app.toml"), "--storage-root", *scoutState, "--lexicon-app", lexiconApp, "--lexicon-storage-root", *lexiconState}
	for _, root := range roots {
		scoutArgs = append(scoutArgs, "--allow-root", root)
	}
	scoutArgs = append(scoutArgs, "review")
	scout, err := runGo(ctx, filepath.Join(*scoutRoot, "cmd", "scout"), scoutArgs...)
	if err != nil {
		return fmt.Errorf("scout review: %w", err)
	}
	scoutFile := filepath.Join(*artifacts, "scout-report.json")
	if err = writeNew(scoutFile, scout); err != nil {
		return err
	}
	receipts, err := runGo(ctx, filepath.Join(*receiptsRoot, "cmd", "receipts"), "--app", filepath.Join(*receiptsRoot, "application", "pulp.app.toml"), "--storage-root", *receiptsState, "list")
	if err != nil {
		return fmt.Errorf("receipts list: %w", err)
	}
	receiptsFile := filepath.Join(*artifacts, "receipts-list.json")
	if err = writeNew(receiptsFile, receipts); err != nil {
		return err
	}
	inspectorArgs := []string{"--" + *mode, "--lexicon-snapshot-file", lexiconFile, "--scout-report-file", scoutFile, "--receipts-list-file", receiptsFile}
	if *contextManifest != "" {
		inspectorArgs = append(inspectorArgs, "--context-manifest-file", *contextManifest)
	}
	result, err := runGo(ctx, filepath.Join(*inspectorRoot, "cmd", "inspector"), inspectorArgs...)
	if err != nil {
		return fmt.Errorf("inspector %s: %w", *mode, err)
	}
	_, err = os.Stdout.Write(result)
	return err
}

func runGo(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "go", append([]string{"run", "."}, args...)...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err == nil {
		return output, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return nil, fmt.Errorf("%w\n%s", err, strings.TrimSpace(string(exit.Stderr)))
	}
	return nil, err
}
func writeNew(path string, value []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("artifact %s: %w", path, err)
	}
	defer f.Close()
	_, err = f.Write(value)
	return err
}

type listFlag []string

func (f *listFlag) String() string         { return strings.Join(*f, ",") }
func (f *listFlag) Set(value string) error { *f = append(*f, value); return nil }
