package inspector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
)

// ScopePolicy is a declared visibility budget, never a hidden gate. Lens
// reports expansion for human review but does not stop the worker or mutate
// its files.
type ScopePolicy struct {
	Version          string   `json:"version"`
	ExpectedFilesMin int      `json:"expected_files_min,omitempty"`
	ExpectedFilesMax int      `json:"expected_files_max,omitempty"`
	AllowedPaths     []string `json:"allowed_paths,omitempty"`
}

type ScopeStatus struct {
	State            string   `json:"state"` // within, expanded, unknown
	ExpectedFilesMin int      `json:"expected_files_min,omitempty"`
	ExpectedFilesMax int      `json:"expected_files_max,omitempty"`
	ObservedFiles    []string `json:"observed_files,omitempty"`
	UnexpectedFiles  []string `json:"unexpected_files,omitempty"`
}

type ScopeFileSource struct{ Path string }

func (f ScopeFileSource) Load() (ScopePolicy, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return ScopePolicy{}, err
	}
	var policy ScopePolicy
	if err := json.Unmarshal(data, &policy); err != nil {
		return ScopePolicy{}, err
	}
	if policy.Version != "lens.scope.v1" {
		return ScopePolicy{}, fmt.Errorf("expected lens.scope.v1 policy")
	}
	if policy.ExpectedFilesMin < 0 || policy.ExpectedFilesMax < 0 || (policy.ExpectedFilesMax > 0 && policy.ExpectedFilesMin > policy.ExpectedFilesMax) {
		return ScopePolicy{}, fmt.Errorf("invalid scope file budget")
	}
	return policy, nil
}

type EventScopeSource struct {
	Base   Source
	Policy ScopeFileSource
}

func (s EventScopeSource) Snapshot(ctx context.Context) (Snapshot, error) {
	base, err := s.Base.Snapshot(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	policy, err := s.Policy.Load()
	if err != nil {
		return Snapshot{}, err
	}
	if base.Live == nil {
		return base, nil
	}
	files := map[string]bool{}
	for _, item := range base.Live.ReviewQueue {
		if item.File != "" {
			files[item.File] = true
		}
	}
	status := ScopeStatus{State: "within", ExpectedFilesMin: policy.ExpectedFilesMin, ExpectedFilesMax: policy.ExpectedFilesMax}
	for file := range files {
		status.ObservedFiles = append(status.ObservedFiles, file)
		if len(policy.AllowedPaths) > 0 && !scopeAllows(policy.AllowedPaths, file) {
			status.UnexpectedFiles = append(status.UnexpectedFiles, file)
		}
	}
	sort.Strings(status.ObservedFiles)
	sort.Strings(status.UnexpectedFiles)
	if len(status.UnexpectedFiles) > 0 || (policy.ExpectedFilesMax > 0 && len(status.ObservedFiles) > policy.ExpectedFilesMax) {
		status.State = "expanded"
	}
	base.Live.Scope = &status
	return base, nil
}

func scopeAllows(patterns []string, file string) bool {
	file = strings.TrimPrefix(path.Clean(file), "./")
	for _, pattern := range patterns {
		pattern = strings.TrimPrefix(path.Clean(pattern), "./")
		if strings.HasSuffix(pattern, "/**") && strings.HasPrefix(file, strings.TrimSuffix(pattern, "**")) {
			return true
		}
		matched, _ := path.Match(pattern, file)
		if matched {
			return true
		}
	}
	return false
}
