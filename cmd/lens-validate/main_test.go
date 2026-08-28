package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_LENS_VALIDATE_HELPER") != "1" {
		return
	}
	for index, value := range os.Args {
		if value == "--" {
			os.Args = append([]string{os.Args[0]}, os.Args[index+1:]...)
			main()
			return
		}
	}
	os.Exit(2)
}

func TestValidationWritesStartedAndPassed(t *testing.T) {
	events := filepath.Join(t.TempDir(), "events.jsonl")
	validation := []string{"sh", "-c", "exit 0"}
	if runtime.GOOS == "windows" {
		validation = []string{"cmd", "/c", "exit", "0"}
	}
	args := append([]string{"-test.run=TestHelperProcess", "--", "--events", events, "--id", "check", "--target", "a.go", "--kind", "test"}, validation...)
	command := exec.Command(os.Args[0], args...)
	command.Env = append(os.Environ(), "GO_WANT_LENS_VALIDATE_HELPER=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("validate: %v\n%s", err, output)
	}
	data, err := os.ReadFile(events)
	if err != nil || !strings.Contains(string(data), "validation.started") || !strings.Contains(string(data), "validation.passed") {
		t.Fatalf("events=%q err=%v", data, err)
	}
}
