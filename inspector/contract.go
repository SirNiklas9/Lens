// Package inspector defines Inspector's read-only local projection contract.
//
// Inspector deliberately owns no knowledge, receipt, or Scout state. Adapters
// obtain snapshots from their respective applications and this package creates
// a small, stable presentation projection for a native client.
package inspector

import (
	"context"
	"errors"
	"sort"
	"time"
)

// Source is a read-only adapter. Implementations may call Lexicon's local API,
// read a host-provided snapshot, or adapt another explicitly declared source.
// They must not mutate the source while building a projection.
type Source interface {
	Snapshot(context.Context) (Snapshot, error)
}

// Snapshot is the stable, transport-safe Inspector API. Details are opaque
// source projections so Inspector does not copy or become authoritative for
// Lexicon, Scout, Pith, Receipts, or a host session manifest.
type Snapshot struct {
	Version        string          `json:"version"`
	Generated      time.Time       `json:"generated_at"`
	Active         ActiveScope     `json:"active"`
	Health         Health          `json:"health"`
	Context        Context         `json:"context"`
	Lexicon        any             `json:"lexicon,omitempty"`
	Scout          any             `json:"scout,omitempty"`
	Pith           any             `json:"pith,omitempty"`
	Receipts       any             `json:"receipts,omitempty"`
	Live           *Live           `json:"live,omitempty"`
	Threads        []Thread        `json:"threads,omitempty"`
	Reconciliation *Reconciliation `json:"reconciliation,omitempty"`
	Legacy         *LegacyStatus   `json:"legacy_projx,omitempty"`
}

type ActiveScope struct {
	Directory string   `json:"directory,omitempty"`
	Global    string   `json:"global,omitempty"`
	Workspace string   `json:"workspace,omitempty"`
	Project   string   `json:"project,omitempty"`
	Sources   []string `json:"sources,omitempty"`
}

// Context describes the active model context manifest, not model content.
// Counts/digests let a client explain baseline versus delta without retaining
// prompts or source bodies.
type Context struct {
	SessionID      string `json:"session_id,omitempty"`
	ManifestDigest string `json:"manifest_digest,omitempty"`
	Mode           string `json:"mode,omitempty"` // baseline, delta, or rehydration
	RuleCount      int    `json:"rule_count,omitempty"`
	KnowledgeCount int    `json:"knowledge_count,omitempty"`
	CodeMapVersion string `json:"code_map_version,omitempty"`
}

type Health struct {
	State    string          `json:"state"` // healthy, degraded, unavailable
	Services []ServiceHealth `json:"services"`
}

type ServiceHealth struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
}

// LegacyStatus intentionally includes only values that map to the new model.
// ProjX harness/agent/gate/dispatch state is excluded: Inspector does not
// recreate that harness.
type LegacyStatus struct {
	StoreHealthy *bool  `json:"store_healthy,omitempty"`
	MCPHealthy   *bool  `json:"mcp_healthy,omitempty"`
	HookHealthy  *bool  `json:"hook_healthy,omitempty"`
	CodeMapState string `json:"code_map_state,omitempty"`
}

// Build reads all sources and returns one read-only API projection. A source
// failure is represented as degraded health so a native window remains useful
// while one optional application is unavailable.
func Build(ctx context.Context, sources map[string]Source) Snapshot {
	out := Snapshot{Version: "inspector.snapshot.v1", Generated: time.Now().UTC()}
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		s, err := sources[name].Snapshot(ctx)
		if err != nil {
			out.Health.Services = append(out.Health.Services, ServiceHealth{Name: name, State: "unavailable", Message: err.Error()})
			continue
		}
		merge(&out, s)
		out.Health.Services = append(out.Health.Services, ServiceHealth{Name: name, State: "healthy"})
	}
	if len(out.Health.Services) == 0 {
		out.Health.State = "unavailable"
		return out
	}
	out.Health.State = "healthy"
	for _, service := range out.Health.Services {
		if service.State != "healthy" {
			out.Health.State = "degraded"
			break
		}
	}
	return out
}

func merge(out *Snapshot, in Snapshot) {
	if in.Active.Directory != "" || in.Active.Global != "" || in.Active.Workspace != "" || in.Active.Project != "" {
		out.Active = in.Active
	}
	if in.Context.SessionID != "" || in.Context.ManifestDigest != "" || in.Context.Mode != "" || in.Context.RuleCount != 0 || in.Context.KnowledgeCount != 0 || in.Context.CodeMapVersion != "" {
		out.Context = in.Context
	}
	if in.Lexicon != nil {
		out.Lexicon = in.Lexicon
	}
	if in.Scout != nil {
		out.Scout = in.Scout
	}
	if in.Pith != nil {
		out.Pith = in.Pith
	}
	if in.Receipts != nil {
		out.Receipts = in.Receipts
	}
	if in.Live != nil {
		out.Live = in.Live
	}
	if len(in.Threads) > 0 {
		out.Threads = in.Threads
	}
	if in.Reconciliation != nil {
		out.Reconciliation = in.Reconciliation
	}
	if in.Legacy != nil {
		out.Legacy = in.Legacy
	}
}

// Validate prevents a source adapter from emitting an ambiguous projection.
func (s Snapshot) Validate() error {
	if s.Version != "" && s.Version != "inspector.snapshot.v1" {
		return errors.New("inspector: unsupported snapshot version")
	}
	if s.Context.Mode != "" && s.Context.Mode != "baseline" && s.Context.Mode != "delta" && s.Context.Mode != "rehydration" {
		return errors.New("inspector: invalid context mode")
	}
	return nil
}
