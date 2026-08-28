package inspector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// Intent is a concise, user/worker-declared task contract. It is deliberately
// not hidden model reasoning. Lens compares it with observable evidence only.
type Intent struct {
	Version             string   `json:"version"`
	TaskID              string   `json:"task_id,omitempty"`
	Goal                string   `json:"goal,omitempty"`
	ExpectedPaths       []string `json:"expected_paths,omitempty"`
	ExpectedSymbols     []string `json:"expected_symbols,omitempty"`
	RequiredValidations []string `json:"required_validations,omitempty"`
}

type Reconciliation struct {
	State              string   `json:"state"` // aligned, review_required
	Goal               string   `json:"goal,omitempty"`
	ObservedFiles      []string `json:"observed_files,omitempty"`
	ObservedSymbols    []string `json:"observed_symbols,omitempty"`
	UnexpectedFiles    []string `json:"unexpected_files,omitempty"`
	MissingSymbols     []string `json:"missing_symbols,omitempty"`
	MissingValidations []string `json:"missing_validations,omitempty"`
	FailedValidations  []string `json:"failed_validations,omitempty"`
}

type IntentFileSource struct{ Path string }

func (f IntentFileSource) Load() (Intent, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return Intent{}, err
	}
	var intent Intent
	if err := json.Unmarshal(data, &intent); err != nil {
		return Intent{}, err
	}
	if intent.Version != "lens.intent.v1" {
		return Intent{}, fmt.Errorf("expected lens.intent.v1")
	}
	return intent, nil
}

type EventIntentSource struct {
	Base   Source
	Intent IntentFileSource
}

func (s EventIntentSource) Snapshot(ctx context.Context) (Snapshot, error) {
	base, err := s.Base.Snapshot(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	intent, err := s.Intent.Load()
	if err != nil {
		return Snapshot{}, err
	}
	if base.Live == nil {
		return base, nil
	}
	r := Reconciliation{State: "aligned", Goal: intent.Goal}
	files, symbols, passed, failed := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, item := range base.Live.ReviewQueue {
		if item.File != "" {
			files[item.File] = true
		}
		if item.Symbol != "" {
			symbols[item.Symbol] = true
		}
	}
	for _, value := range base.Live.Validation {
		if value.State == "PASSED" {
			passed[value.Kind] = true
		}
		if value.State == "FAILED" {
			failed[value.Kind] = true
		}
	}
	for file := range files {
		r.ObservedFiles = append(r.ObservedFiles, file)
		if len(intent.ExpectedPaths) > 0 && !scopeAllows(intent.ExpectedPaths, file) {
			r.UnexpectedFiles = append(r.UnexpectedFiles, file)
		}
	}
	for symbol := range symbols {
		r.ObservedSymbols = append(r.ObservedSymbols, symbol)
	}
	for _, expected := range intent.ExpectedSymbols {
		if !symbols[expected] {
			r.MissingSymbols = append(r.MissingSymbols, expected)
		}
	}
	for _, required := range intent.RequiredValidations {
		if !passed[required] {
			r.MissingValidations = append(r.MissingValidations, required)
		}
		if failed[required] {
			r.FailedValidations = append(r.FailedValidations, required)
		}
	}
	for _, list := range [][]string{r.ObservedFiles, r.ObservedSymbols, r.UnexpectedFiles, r.MissingSymbols, r.MissingValidations, r.FailedValidations} {
		sort.Strings(list)
	}
	if len(r.UnexpectedFiles) > 0 || len(r.MissingSymbols) > 0 || len(r.MissingValidations) > 0 || len(r.FailedValidations) > 0 {
		r.State = "review_required"
	}
	base.Reconciliation = &r
	return base, nil
}
