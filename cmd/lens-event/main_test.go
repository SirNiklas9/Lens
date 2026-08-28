package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublishesOneExplicitEvent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events", "live.jsonl")
	command := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", "-out", path, "-id", "change-1", "-kind", "symbol.touched", "-file", "internal/link.go", "-symbol", "prepare")
	command.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("lens-event: %v\n%s", err, output)
	}
	got, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(got), `"kind":"symbol.touched"`) {
		t.Fatalf("event=%q err=%v", got, err)
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for i, value := range args {
		if value == "--" {
			os.Args = append([]string{args[0]}, args[i+1:]...)
			main()
			return
		}
	}
	os.Exit(2)
}
