package inspector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Live is a bounded, read-only projection of normalized worker events. Lens
// owns neither the event log nor source files: an Agent/Event Adapter supplies
// explicit JSONL and Lens derives an immediately reviewable view from it.
type Live struct {
	Version     string       `json:"version"`
	TaskID      string       `json:"task_id,omitempty"`
	Goal        string       `json:"goal,omitempty"`
	Current     *Activity    `json:"current,omitempty"`
	Recent      []Event      `json:"recent,omitempty"`
	ReviewQueue []ReviewItem `json:"review_queue,omitempty"`
	Validation  []Validation `json:"validation,omitempty"`
	Scope       *ScopeStatus `json:"scope,omitempty"`
}

type Activity struct {
	Kind      string `json:"kind"`
	File      string `json:"file,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Symbol    string `json:"symbol,omitempty"`
	Why       string `json:"why,omitempty"`
}

// Event is provider-neutral. The adapter may derive it from host hooks, Git,
// filesystem notifications, or a direct worker integration. Source snippets,
// prompts, and model reasoning are deliberately absent.
type Event struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id,omitempty"`
	Kind      string `json:"kind"`
	At        string `json:"at,omitempty"`
	File      string `json:"file,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Symbol    string `json:"symbol,omitempty"`
	Why       string `json:"why,omitempty"`
	Outcome   string `json:"outcome,omitempty"`
	ReviewID  string `json:"review_id,omitempty"`
}

// ValidateEvent verifies the small, provider-neutral event envelope before an
// adapter publishes it. It intentionally has no provider-specific fields and
// accepts no source body, prompt, or model reasoning.
func ValidateEvent(event Event) error {
	if strings.TrimSpace(event.ID) == "" || strings.TrimSpace(event.Kind) == "" {
		return fmt.Errorf("live event id and kind are required")
	}
	return nil
}

// AppendEvent performs one explicit durable append to a caller-selected event
// stream. It is used by optional adapters; Lens itself never writes an event
// merely because a view was opened or refreshed.
func AppendEvent(path string, event Event) error {
	if err := EnsureEventStream(path); err != nil {
		return err
	}
	if err := ValidateEvent(event); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() > 0 {
		if _, err := file.Seek(-1, 2); err != nil {
			return err
		}
		last := []byte{0}
		if _, err := file.Read(last); err != nil {
			return err
		}
		if last[0] != '\n' {
			if _, err := file.Seek(0, 2); err != nil {
				return err
			}
			if _, err := file.Write([]byte{'\n'}); err != nil {
				return err
			}
		}
	}
	if _, err := file.Seek(0, 2); err != nil {
		return err
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = file.Write(append(payload, '\n'))
	return err
}

// EnsureEventStream creates an explicit empty event stream when needed. A
// clean repository is still a healthy observation result, not a missing Lens
// source. It never chooses a path for the caller.
func EnsureEventStream(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("event output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	return file.Close()
}

type ReviewItem struct {
	ID        string `json:"id"`
	State     string `json:"state"` // NEW, CHECKING, FAILED, PASSING, REVIEWED, ACCEPTED, REJECTED
	File      string `json:"file,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Symbol    string `json:"symbol,omitempty"`
	Why       string `json:"why,omitempty"`
	EventID   string `json:"event_id,omitempty"`
}

type Validation struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	State   string `json:"state"` // CHECKING, PASSED, FAILED
	Target  string `json:"target,omitempty"`
	Message string `json:"message,omitempty"`
}
