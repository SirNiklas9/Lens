package inspector

// AIContext is a bounded, provider-neutral read model for an AI host. It is
// derived from inspector.snapshot.v1 and deliberately excludes record bodies,
// prompts, patches, transcripts, and model output. A host can use it to decide
// what to fetch from the owning application without Inspector becoming a
// second knowledge store.

import "fmt"

const aiContextVersion = "inspector.ai-context.v1"
const maxAIRefs = 32

type AIContext struct {
	Version  string       `json:"version"`
	Active   ActiveScope  `json:"active"`
	Context  Context      `json:"context"`
	Health   Health       `json:"health"`
	Lexicon  *KnowledgeAI `json:"lexicon,omitempty"`
	Scout    *ScoutAI     `json:"scout,omitempty"`
	Pith     *PithAI      `json:"pith,omitempty"`
	Receipts *ReceiptsAI  `json:"receipts,omitempty"`
}

type Reference struct {
	ID        string `json:"id,omitempty"`
	Scope     string `json:"scope,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Key       string `json:"key,omitempty"`
	State     string `json:"state,omitempty"`
	Outcome   string `json:"outcome,omitempty"`
}

type KnowledgeAI struct {
	RecordCount int         `json:"record_count"`
	RuleCount   int         `json:"rule_count"`
	ReviewCount int         `json:"review_count"`
	Review      []Reference `json:"review,omitempty"`
	Truncated   bool        `json:"truncated,omitempty"`
}

type ScoutAI struct {
	Mode       string      `json:"mode,omitempty"`
	Candidates int         `json:"candidates"`
	Proposals  int         `json:"proposals"`
	Skipped    int         `json:"skipped"`
	Items      []Reference `json:"items,omitempty"`
	Truncated  bool        `json:"truncated,omitempty"`
}

type PithAI struct {
	PreviewCount int         `json:"preview_count"`
	Previews     []Reference `json:"previews,omitempty"`
	Truncated    bool        `json:"truncated,omitempty"`
}

type ReceiptsAI struct {
	Count     int         `json:"count"`
	Recent    []Reference `json:"recent,omitempty"`
	Truncated bool        `json:"truncated,omitempty"`
}

// BuildAIContext is pure and bounded. It only selects identity/status fields
// from opaque source snapshots, never copies bodies or asks another service
// for more data.
func BuildAIContext(snapshot Snapshot) (AIContext, error) {
	if err := snapshot.Validate(); err != nil {
		return AIContext{}, err
	}
	out := AIContext{Version: aiContextVersion, Active: snapshot.Active, Context: snapshot.Context, Health: snapshot.Health}
	if snapshot.Lexicon != nil {
		out.Lexicon = knowledgeSummary(snapshot.Lexicon)
	}
	if snapshot.Scout != nil {
		out.Scout = scoutSummary(snapshot.Scout)
	}
	if snapshot.Pith != nil {
		out.Pith = pithSummary(snapshot.Pith)
	}
	if snapshot.Receipts != nil {
		out.Receipts = receiptsSummary(snapshot.Receipts)
	}
	return out, nil
}

func knowledgeSummary(value any) *KnowledgeAI {
	m := object(value)
	out := &KnowledgeAI{RecordCount: len(array(m["records"])), RuleCount: len(array(m["rules"])), ReviewCount: len(array(m["review"]))}
	for _, row := range array(m["review"]) {
		r := object(row)
		record, decision := object(r["record"]), object(r["decision"])
		ref := reference(record)
		ref.State = text(decision["state"])
		out.Review, out.Truncated = appendBounded(out.Review, ref, out.Truncated)
	}
	return out
}

func scoutSummary(value any) *ScoutAI {
	m := object(value)
	out := &ScoutAI{Mode: text(m["mode"]), Candidates: number(m["candidates"]), Proposals: number(m["proposals"]), Skipped: number(m["skipped"])}
	for _, row := range array(m["items"]) {
		r := object(row)
		ref := Reference{ID: text(r["record_id"]), Scope: text(r["scope"])}
		decision, proposal := object(r["decision"]), object(r["proposal"])
		ref.State = text(decision["state"])
		ref.Outcome = text(proposal["outcome"])
		out.Items, out.Truncated = appendBounded(out.Items, ref, out.Truncated)
	}
	return out
}

func pithSummary(value any) *PithAI {
	m := object(value)
	rows := array(m["previews"])
	if len(rows) == 0 {
		rows = array(m["proposals"])
	}
	out := &PithAI{PreviewCount: len(rows)}
	for _, row := range rows {
		out.Previews, out.Truncated = appendBounded(out.Previews, reference(object(row)), out.Truncated)
	}
	return out
}

func receiptsSummary(value any) *ReceiptsAI {
	m := object(value)
	rows := array(m["receipts"])
	out := &ReceiptsAI{Count: len(rows)}
	for _, row := range rows {
		out.Recent, out.Truncated = appendBounded(out.Recent, reference(object(row)), out.Truncated)
	}
	return out
}

func appendBounded(rows []Reference, value Reference, truncated bool) ([]Reference, bool) {
	if len(rows) >= maxAIRefs {
		return rows, true
	}
	return append(rows, value), truncated
}
func reference(m map[string]any) Reference {
	return Reference{ID: text(m["id"]), Scope: text(m["scope"]), Namespace: text(m["namespace"]), Key: text(m["key"]), State: text(m["state"]), Outcome: text(m["outcome"])}
}
func object(value any) map[string]any {
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}
func array(value any) []any {
	if rows, ok := value.([]any); ok {
		return rows
	}
	return nil
}
func text(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}
func number(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case float32:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return 0
	}
}
