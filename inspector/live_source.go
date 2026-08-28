package inspector

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// EventFileSource reads one explicit JSONL event stream. It intentionally does
// not watch a directory or start a background process. Callers decide when to
// refresh it, keeping Lens lightweight and host-neutral.
type EventFileSource struct{ Path string }

func (f EventFileSource) Snapshot(context.Context) (Snapshot, error) {
	file, err := os.Open(f.Path)
	if err != nil {
		return Snapshot{}, err
	}
	defer file.Close()
	live := &Live{Version: "lens.live.v1"}
	items := map[string]*ReviewItem{}
	var order []string
	s := bufio.NewScanner(file)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for line := 1; s.Scan(); line++ {
		payload := s.Bytes()
		// PowerShell may prefix UTF-8 text with a BOM. JSONL remains portable;
		// tolerate that marker only at the beginning of the first record.
		if line == 1 {
			payload = bytes.TrimPrefix(payload, []byte{0xef, 0xbb, 0xbf})
		}
		var event Event
		if err := json.Unmarshal(payload, &event); err != nil {
			return Snapshot{}, fmt.Errorf("live event line %d: %w", line, err)
		}
		if err := ValidateEvent(event); err != nil {
			return Snapshot{}, fmt.Errorf("live event line %d: %w", line, err)
		}
		applyLive(live, items, &order, event)
	}
	if err := s.Err(); err != nil {
		return Snapshot{}, err
	}
	for _, id := range order {
		live.ReviewQueue = append(live.ReviewQueue, *items[id])
	}
	return Snapshot{Version: "inspector.snapshot.v1", Live: live}, nil
}

func applyLive(live *Live, items map[string]*ReviewItem, order *[]string, event Event) {
	if event.TaskID != "" {
		live.TaskID = event.TaskID
	}
	if event.Kind == "task.updated" && event.Why != "" {
		live.Goal = event.Why
	}
	if event.Kind == "edit.started" {
		live.Current = &Activity{Kind: event.Kind, File: event.File, StartLine: event.StartLine, EndLine: event.EndLine, Symbol: event.Symbol, Why: event.Why}
	}
	if event.Kind == "edit.completed" || event.Kind == "task.completed" {
		live.Current = nil
	}
	if event.Kind == "file.modified" || event.Kind == "file.created" || event.Kind == "file.deleted" || event.Kind == "symbol.touched" {
		id := event.ID
		item := &ReviewItem{ID: id, State: "NEW", File: event.File, StartLine: event.StartLine, EndLine: event.EndLine, Symbol: event.Symbol, Why: event.Why, EventID: event.ID}
		items[id] = item
		*order = append(*order, id)
		if len(*order) > 64 {
			oldest := (*order)[0]
			*order = (*order)[1:]
			delete(items, oldest)
		}
	}
	if event.Kind == "validation.started" || event.Kind == "validation.passed" || event.Kind == "validation.failed" {
		state := map[string]string{"validation.started": "CHECKING", "validation.passed": "PASSED", "validation.failed": "FAILED"}[event.Kind]
		live.Validation = append(live.Validation, Validation{ID: event.ID, Kind: event.Symbol, State: state, Target: event.File, Message: event.Why})
		for _, item := range items {
			if event.File != "" && item.File == event.File {
				if state == "FAILED" {
					item.State = "FAILED"
				} else if state == "PASSED" && item.State == "NEW" {
					item.State = "PASSING"
				}
			}
		}
	}
	if event.Kind == "reviewed" || event.Kind == "review.accepted" || event.Kind == "review.rejected" {
		if item := items[event.ReviewID]; item != nil {
			item.State = map[string]string{
				"reviewed":        "REVIEWED",
				"review.accepted": "ACCEPTED",
				"review.rejected": "REJECTED",
			}[event.Kind]
		}
	}
	live.Recent = append(live.Recent, event)
	if len(live.Recent) > 64 {
		live.Recent = live.Recent[len(live.Recent)-64:]
	}
}
