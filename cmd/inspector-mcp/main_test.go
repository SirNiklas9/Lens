package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestMCPInitializeAndTools(t *testing.T) {
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" + `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n")
	var output bytes.Buffer
	if err := serve(input, &output); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("responses=%s", output.String())
	}
	if !strings.Contains(lines[0], `"name":"lens"`) || !strings.Contains(lines[1], `"lens_ai_context"`) {
		t.Fatalf("responses=%s", output.String())
	}
	for _, line := range lines {
		var value map[string]any
		if err := json.Unmarshal([]byte(line), &value); err != nil {
			t.Fatal(err)
		}
	}
}
