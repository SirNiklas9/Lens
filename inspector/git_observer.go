package inspector

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// GitChanges returns a bounded, source-body-free view of tracked file changes.
// It is deliberately an observer: it neither stages, resets, nor modifies a
// working tree. Callers may run it once or in their own foreground loop.
func GitChanges(root string) ([]Event, error) {
	states := map[string]string{}
	ranges := map[string][]lineRange{}
	for _, args := range [][]string{{"diff", "--name-status"}, {"diff", "--cached", "--name-status"}} {
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		output, err := command.Output()
		if err != nil {
			return nil, fmt.Errorf("git observation: %w", err)
		}
		for file, state := range parseNameStatus(string(output)) {
			states[file] = state
		}
	}
	for _, args := range [][]string{{"diff", "--no-ext-diff", "--unified=0"}, {"diff", "--cached", "--no-ext-diff", "--unified=0"}} {
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		output, err := command.Output()
		if err != nil {
			return nil, fmt.Errorf("git observation: %w", err)
		}
		for file, hunks := range parseUnifiedZero(string(output)) {
			ranges[file] = append(ranges[file], hunks...)
		}
	}
	command := exec.Command("git", "-C", root, "ls-files", "--others", "--exclude-standard")
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("git observation: %w", err)
	}
	for _, file := range strings.Fields(string(output)) {
		if _, tracked := states[file]; !tracked {
			states[file] = "A"
		}
	}
	files := make([]string, 0, len(states))
	for file := range states {
		files = append(files, file)
	}
	sort.Strings(files)
	events := make([]Event, 0, len(files))
	for _, file := range files {
		state := states[file]
		kind := map[string]string{"A": "file.created", "D": "file.deleted"}[state]
		if kind == "" {
			kind = "file.modified"
		}
		hunks := ranges[file]
		if len(hunks) == 0 {
			events = append(events, gitEvent(state, kind, file, 0, 0))
			continue
		}
		for _, hunk := range hunks {
			events = append(events, gitEvent(state, kind, file, hunk.start, hunk.end))
		}
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].File != events[j].File {
			return events[i].File < events[j].File
		}
		return events[i].StartLine < events[j].StartLine
	})
	return events, nil
}

func gitEvent(state, kind, file string, start, end int) Event {
	sum := sha256.Sum256([]byte(state + "\x00" + file + "\x00" + strconv.Itoa(start) + "\x00" + strconv.Itoa(end)))
	return Event{ID: "git-" + hex.EncodeToString(sum[:8]), Kind: kind, File: filepath.ToSlash(file), StartLine: start, EndLine: end, At: time.Now().UTC().Format(time.RFC3339Nano), Why: "observed by Git working-tree diff"}
}

type lineRange struct{ start, end int }

var hunkPattern = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// parseUnifiedZero returns changed line ranges in the *new* file. Deleted-only
// hunks use their insertion point so Lens can select the enclosing declaration
// rather than inventing a line that no longer exists.
func parseUnifiedZero(diff string) map[string][]lineRange {
	result := map[string][]lineRange{}
	var file string
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			file = strings.TrimPrefix(line, "+++ b/")
			continue
		}
		match := hunkPattern.FindStringSubmatch(line)
		if len(match) == 0 || file == "" {
			continue
		}
		start, _ := strconv.Atoi(match[1])
		count := 1
		if match[2] != "" {
			count, _ = strconv.Atoi(match[2])
		}
		end := start + count - 1
		if count == 0 {
			end = start
		}
		result[file] = append(result[file], lineRange{start: start, end: end})
	}
	return result
}

func parseNameStatus(output string) map[string]string {
	result := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) < 2 || parts[0] == "" {
			continue
		}
		state, file := parts[0], parts[len(parts)-1]
		if len(state) > 0 {
			state = state[:1]
		}
		if file != "" {
			result[file] = state
		}
	}
	return result
}
