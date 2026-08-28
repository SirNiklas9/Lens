package inspector

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type sourceFunc func(context.Context) (Snapshot, error)

func (f sourceFunc) Snapshot(c context.Context) (Snapshot, error) { return f(c) }

func TestBuildMergesReadOnlySourcesAndDegrades(t *testing.T) {
	s := Build(context.Background(), map[string]Source{
		"lexicon": sourceFunc(func(context.Context) (Snapshot, error) {
			return Snapshot{Active: ActiveScope{Project: "lexicon"}, Context: Context{Mode: "delta", RuleCount: 2}, Lexicon: map[string]any{"records": 3}}, nil
		}),
		"scout": sourceFunc(func(context.Context) (Snapshot, error) { return Snapshot{}, errors.New("offline") }),
	})
	if s.Health.State != "degraded" || s.Active.Project != "lexicon" || s.Context.Mode != "delta" {
		t.Fatalf("snapshot=%#v", s)
	}
	if s.Lexicon == nil || len(s.Health.Services) != 2 {
		t.Fatalf("merge/services=%#v", s)
	}
}

func TestHandlerIsReadOnlyAndStable(t *testing.T) {
	h := Handler(map[string]Source{"lexicon": sourceFunc(func(context.Context) (Snapshot, error) { return Snapshot{Lexicon: map[string]any{"records": 1}}, nil })})
	r := httptest.NewRequest(http.MethodGet, "/v1/snapshot", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	var s Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if s.Version != "inspector.snapshot.v1" || s.Lexicon == nil {
		t.Fatalf("snapshot=%#v", s)
	}
	r = httptest.NewRequest(http.MethodPost, "/v1/snapshot", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	} // endpoint has no mutation semantics
	for _, p := range []string{"/v1/write", "/v1/apply"} {
		r = httptest.NewRequest(http.MethodGet, p, nil)
		w = httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s exposed status %d", p, w.Code)
		}
	}
	r = httptest.NewRequest(http.MethodGet, "/v1/ai-context", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	var ai AIContext
	if err := json.Unmarshal(w.Body.Bytes(), &ai); err != nil {
		t.Fatal(err)
	}
	if ai.Version != aiContextVersion {
		t.Fatalf("ai=%#v", ai)
	}
}
