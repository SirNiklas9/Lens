package inspector_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestLiveReviewLoopAcceptance exercises the portable live-review path as a
// user would: Git hunk observation, Atlas symbol resolution, declared
// validation, explicit review acceptance, scope budget, and intent evidence.
// It uses only a disposable repository and Lens JSONL/JSON artifacts.
func TestLiveReviewLoopAcceptance(t *testing.T) {
	if os.Getenv("RUN_LENS_LIVE_E2E") != "1" {
		t.Skip("set RUN_LENS_LIVE_E2E=1")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0700); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "git", "init", "-q")
	run(t, repo, "git", "config", "user.email", "lens@example.invalid")
	run(t, repo, "git", "config", "user.name", "Lens")
	file := filepath.Join(repo, "sample.go")
	if err := os.WriteFile(file, []byte("func Alpha() {}\n\n\n\n\n\n\n\n\nfunc Bravo() {}\n\n\n\n\n\n\n\n\nfunc Charlie() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "git", "add", "sample.go")
	run(t, repo, "git", "commit", "-qm", "initial")
	if err := os.WriteFile(file, []byte("func Alpha() {}\n\n\n\n\n\n\n\n\nfunc Bravo() { println(\"changed\") }\n\n\n\n\n\n\n\n\nfunc Charlie() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	events := filepath.Join(root, "events.jsonl")
	atlas := filepath.Join(root, "atlas.json")
	policy := filepath.Join(root, "scope.json")
	intent := filepath.Join(root, "intent.json")
	if err := os.WriteFile(atlas, []byte(`{"version":"atlas.snapshot.v1","root":"fixture","symbols":[{"id":"a","name":"Alpha","kind":"func","file":"sample.go","line":1},{"id":"b","name":"Bravo","kind":"func","file":"sample.go","line":10},{"id":"c","name":"Charlie","kind":"func","file":"sample.go","line":20}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, []byte(`{"version":"lens.scope.v1","expected_files_max":1,"allowed_paths":["sample.go"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(intent, []byte(`{"version":"lens.intent.v1","goal":"change Bravo only","expected_paths":["sample.go"],"expected_symbols":["Bravo"],"required_validations":["test"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	project, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// The package root is the Inspector repository during go test.
	run(t, project, "go", "run", "./cmd/lens-observe", "--root", repo, "--out", events)
	run(t, project, "go", "run", "./cmd/lens-validate", "--events", events, "--id", "test", "--target", "sample.go", "--kind", "test", "cmd", "/c", "exit", "0")
	run(t, project, "go", "run", "./cmd/lens-review", "--events", events, "--review-id", findReviewID(t, events), "--decision", "accepted", "--actor", "tester", "--scope", "project")
	out := run(t, project, "go", "run", "./cmd/inspector", "--snapshot", "--live-events-file", events, "--atlas-snapshot-file", atlas, "--scope-policy-file", policy, "--intent-file", intent)
	for _, need := range []string{`"symbol":"Bravo"`, `"state":"ACCEPTED"`, `"state":"aligned"`, `"state":"PASSED"`} {
		if !bytes.Contains(out, []byte(need)) {
			t.Fatalf("missing %s in %s", need, out)
		}
	}
}

func findReviewID(t *testing.T, events string) string {
	data, err := os.ReadFile(events)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, `"file":"sample.go"`) && strings.Contains(line, `"kind":"file.modified"`) {
			start := strings.Index(line, `"id":"`) + len(`"id":"`)
			end := strings.Index(line[start:], `"`)
			if start >= len(`"id":"`) && end >= 0 {
				return line[start : start+end]
			}
		}
	}
	t.Fatalf("no Git review ID in %s", data)
	return ""
}

func run(t *testing.T, dir, command string, args ...string) []byte {
	t.Helper()
	c := exec.Command(command, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GOCACHE="+filepath.Join(t.TempDir(), "cache"))
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", command, strings.Join(args, " "), err, out)
	}
	return out
}
