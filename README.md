# Lens

Lens is a standalone local read-only application. Its future native window
will embed a UI over one local API/event stream and explain active scope,
context deltas, rules/freshness, Scry proposals, Splice previews, Trace, and
test outcomes. It is not a browser dashboard or a ProjX harness integration.

Lens owns no data. Its `inspector.snapshot.v1` contract accepts explicit,
read-only adapters for Lexicon, Scry, Splice, Trace, and a host context
manifest. The first local adapter reads a supplied snapshot JSON file; a
production Lexicon adapter will call its declared local read-only snapshot API.

## Lens live review MVP

Lens can project an explicit, provider-neutral JSONL event stream into a live
task summary, bounded changed-symbol review queue, and validation status. It
does not run a watcher, re-index a repository, or start Atlas in the
background. An agent adapter (Codex hooks, a filesystem/Git bridge, or another
worker integration) writes the event stream; Lens only reads it when refreshed.

`lens-event` is the intentionally tiny provider-neutral publisher for that
stream. It appends one validated event to an explicit path and exits—there is
no hidden daemon, repository watcher, or required Codex-specific adapter:

```powershell
lens-event --out C:\state\lens\events.jsonl --id change-1 --kind symbol.touched `
  --task link-fix --file internal/links/apply.go --start-line 71 --end-line 89 `
  --symbol prepareDestination --why "Validation now precedes removal"
```

For an explicit human decision, publish a separate transition rather than
changing a validation result in place:

```powershell
lens-event --out C:\state\lens\events.jsonl --id review-1 --kind review.accepted --review-id change-1
```

Any worker can emit the same public event shape. A future filesystem/Git or
Codex adapter may derive these events, but Lens never requires the worker to
route edits through Splice or an AI Deck harness.

For an opt-in Git-derived file view, use the standalone foreground observer.
It reads unstaged and staged name/status diffs, appends normalized
`file.created` / `file.modified` / `file.deleted` events to the explicit
stream, and never stages, resets, or writes to the observed repository:

```powershell
lens-observe --root C:\dev\Pulp --out C:\state\lens\events.jsonl
# Deliberate foreground polling; stop with Ctrl+C.
lens-observe --root C:\dev\Pulp --out C:\state\lens\events.jsonl --interval 5s
```

This is file-level evidence only. Resolving changed ranges into symbols stays
an explicit bounded Atlas operation. When you already have an Atlas export,
provide it to Lens to resolve changed files into real declarations without
starting a large indexer on its own:

```powershell
lens --snapshot --live-events-file C:\state\lens\events.jsonl `
  --atlas-snapshot-file C:\state\pulp-atlas.json
```

Only symbols present in the supplied `atlas.snapshot.v1` are surfaced; any
file Atlas cannot map stays a file-level review item.

If a repository has no committed baseline yet, Git correctly reports its
untracked files as newly created. Lens exposes that honestly as a review queue,
rather than pretending it has a prior revision to compare.

```sh
lens --snapshot --live-events-file ./lens-events.jsonl
# or serve it to Lens.nvim
lens --live-events-file ./lens-events.jsonl
```

Lens can also render an explicit bounded structural map produced by Atlas:

```powershell
atlas --root C:\dev\Pulp --output C:\state\pulp-atlas.json snapshot
atlas --snapshot C:\state\pulp-atlas.json --symbol SomeEntry thread > C:\state\pulp-thread.json
lens --snapshot --thread-file C:\state\pulp-thread.json
```

This is the first `Thread` path into Lens: source-backed declarations and call
edges only. It is not an invented behavioral narrative and it does not make
Atlas a background process.

Each line is one `lens.live.event.v1`-style object. Required fields are `id`
and `kind`; source bodies, prompts, and chain-of-thought are never included.

```json
{"id":"task-1","kind":"task.updated","task_id":"link-fix","why":"Validate source before mutation"}
{"id":"edit-1","kind":"edit.started","file":"internal/links/apply.go","symbol":"prepareDestination","why":"Validate source first"}
{"id":"change-1","kind":"symbol.touched","file":"internal/links/apply.go","start_line":71,"end_line":89,"symbol":"prepareDestination","why":"Validation now precedes removal"}
{"id":"test-1","kind":"validation.passed","file":"internal/links/apply.go","symbol":"go test ./internal/links"}
```

`file.modified`, `file.created`, `file.deleted`, and `symbol.touched` create
review items. `validation.started`, `validation.passed`, and
`validation.failed` update the matching file's review state. The event source
is deliberately bounded to the most recent 64 events per projection.

Human review is a separate normalized transition: `reviewed`,
`review.accepted`, or `review.rejected` must carry the original item's
`review_id`. This makes a green validation result `PASSING`, not silently
`ACCEPTED`.

## Durable review packet

`lens-review` turns an explicit human decision into both a Lens transition and,
when given Trace paths, an immutable Trace receipt. Trace is written first; if
it rejects the receipt, the Lens event is not appended. Nothing is automatic.

```powershell
lens-review --events C:\state\lens\events.jsonl --review-id change-1 `
  --decision accepted --actor nicholas --scope project `
  --trace C:\path\to\trace.exe `
  --trace-app C:\path\to\trace\application\pulp.app.toml `
  --trace-storage-root C:\state\trace
```

The receipt records opaque review identity, actor, scope, outcome, timestamp,
and an evidence reference. It never stores source bodies, diffs, prompts, or
model reasoning.

## Declared validation packet

`lens-validate` runs one command that *you explicitly provide*, emitting a
`validation.started` event and then `validation.passed` or
`validation.failed`. It uses an argument array directly—no shell—and does not
guess, schedule, or repeatedly launch project tests.

```powershell
lens-validate --events C:\state\lens\events.jsonl --id links-focused-test `
  --target internal/links/apply.go --kind test --cwd C:\dev\Pulp `
  go test ./internal/links
```

The live review queue becomes `CHECKING`, then `PASSING` or `FAILED`; it never
becomes human-accepted as a side effect of green tests.

Pass the same explicit Trace paths as `lens-review` to retain an immutable
validation receipt. Trace receives the command kind/outcome and opaque Lens
references, never a command transcript, source body, or prompt:

```powershell
lens-validate --events C:\state\lens\events.jsonl --id links-focused-test `
  --target internal/links/apply.go --kind test --cwd C:\dev\Pulp `
  --trace C:\path\to\trace.exe --trace-app C:\path\to\trace\application\pulp.app.toml `
  --trace-storage-root C:\state\trace --actor codex --scope project `
  go test ./internal/links
```

## Scope-budget packet

Lens can show an explicit, non-blocking scope budget alongside live changes:

```json
{
  "version": "lens.scope.v1",
  "expected_files_min": 2,
  "expected_files_max": 4,
  "allowed_paths": ["internal/links/**", "tests/**"]
}
```

```powershell
lens --snapshot --live-events-file C:\state\lens\events.jsonl `
  --scope-policy-file C:\state\lens\scope.json
```

If an observed change leaves the declared paths or exceeds the file budget,
Lens reports `Scope: expanded`. It does not prevent the worker from continuing;
the purpose is human visibility and review, not a new agent harness gate.

## Intent reconciliation packet

Lens can compare a short declared task contract with observed changed symbols,
paths, and validation evidence. This is not chain-of-thought capture and Lens
never infers intent on its own:

```json
{
  "version": "lens.intent.v1",
  "task_id": "link-fix",
  "goal": "Validate the source before changing destination state",
  "expected_paths": ["internal/links/**", "tests/**"],
  "expected_symbols": ["prepareDestination", "TestInvalidReplacement"],
  "required_validations": ["test"]
}
```

```powershell
lens --snapshot --live-events-file C:\state\lens\events.jsonl `
  --intent-file C:\state\lens\intent.json
```

`aligned` means the declared evidence is present. `review_required` lists the
specific missing/failed validation, missing expected symbol, or unexpected
file. It is an explanation aid, not an autonomous approval or a worker gate.

## Approved behavior lifecycle packet

An approved Atlas Thread can become a *volatile*, source-backed Lexicon
behavior proposal. Lens creates the proposal only; Lexicon remains the sole
durable store and still requires its explicit capture approval:

```powershell
lens-behavior-capture --thread-file C:\state\pulp-thread.json `
  --source C:\dev\Pulp\abi\envelope.go --scope project `
  --id behavior-step-envelope --key step-envelope --review-at 1787000000000 `
  --output C:\state\behavior-step-envelope.capture.json

# Review first; this performs no write.
lexicon --app C:\path\to\lexicon\application\pulp.app.toml --storage-root C:\state\lexicon `
  capture C:\state\behavior-step-envelope.capture.json

# Explicitly approve the verified proposal.
lexicon --app C:\path\to\lexicon\application\pulp.app.toml --storage-root C:\state\lexicon `
  capture C:\state\behavior-step-envelope.capture.json --apply
```

The resulting record contains the bounded Thread JSON, a source-file SHA-256,
and `refresh.verifier=filesystem`. When its lifecycle review time is due,
Scry observes that declared file and returns a refresh proposal if its digest
changed. Scry cannot rewrite the behavior summary; a revised Thread still goes
through explicit Lexicon capture approval.
For the portable application path, Inspector also accepts the actual JSON
artifacts emitted by Lexicon, Scout, and Receipts. They are explicit handoffs
between independent processes; Inspector does not share their databases or
launch them behind the user's back.

Run a local API:

```powershell
go run ./cmd/inspector --snapshot-file .\sample-snapshot.json
```

Project real application outputs without an install, daemon, or browser:

```powershell
go run ./cmd/inspector --snapshot `
  --lexicon-snapshot-file .\lexicon-snapshot.json `
  --scout-report-file .\scout-report.json `
  --receipts-list-file .\receipts.json `
  --context-manifest-file .\context.json
```

It serves `GET /v1/snapshot` and `GET /v1/health` on `127.0.0.1:7341`. There
are no write routes. Inspector Desktop in [`desktop`](desktop) is the portable
native embedded-web window that consumes this API; Inspector never launches an
external browser.

## AI context projection

`GET /v1/ai-context` and `--ai-context` expose the bounded,
versioned `inspector.ai-context.v1` projection. It is designed for any AI host
to consume directly: active scope, context baseline/delta status, health,
Lexicon review references, Scry proposal summaries, Splice preview identities,
and receipt summaries. It contains no Lexicon bodies, prompts, patches,
transcripts, or model output, and it performs no model/provider call or write.

```powershell
go run ./cmd/inspector --ai-context `
  --lexicon-snapshot-file .\lexicon-snapshot.json `
  --scout-report-file .\scout-report.json `
  --receipts-list-file .\receipts.json `
  --context-manifest-file .\context.json
```

## Portable end-to-end fixture

This is the release-acceptance entrypoint for the current single-machine suite.
It starts real foreground processes in isolated temporary state; it never reads
or writes the canonical Lexicon store, any user configuration, or Codex host
configuration.

From the workspace root on Windows:

```powershell
$env:RUN_PORTABLE_APP_E2E = '1'
$env:GOCACHE = "$PWD\inspector\.gocache-release-acceptance"
go test -C .\inspector -run TestPortableApplicationFlow -count=1 -v
```

The test is intentionally explicit about what it accepts:

| Application | Real-process acceptance behavior |
| --- | --- |
| Lexicon | plans and applies a scoped seed, then proposes and explicitly approves a capture; both records must appear in its snapshot |
| Splice | previews a deterministic bounded replacement without a source write, then explicitly applies it; the target range changes and an unrelated sentinel stays unchanged |
| Scout | reads Lexicon evidence and emits one or more refresh proposals; Lexicon snapshot bytes must remain unchanged |
| Receipts | appends an opaque receipt and lists it from a separate application process (restart proof) |
| Inspector | projects real Lexicon/Scout/Receipts artifacts and emits a bounded `inspector.ai-context.v1` packet |

All state, evidence, source fixtures, providers, caches, and handoff artifacts
are created below Go's test temporary directory. A failure reports the exact
application boundary that violated its contract. This accepts neither native
Desktop lifecycle injection nor multi-machine synchronization; those are host
integration concerns outside the isolated application release suite.

## Portable application handoff

The fixture is verification, not the operational workflow. Run the portable
foreground handoff command to collect a real state projection from separately
runnable source applications. It creates a *new* artifact directory, never
installs anything, never starts a background service, and does not change
Lexicon knowledge.

From the Inspector repository:

```powershell
go run ./cmd/inspector-suite `
  --lexicon-root ..\lexicon --scout-root ..\scout --receipts-root ..\receipts `
  --lexicon-storage-root C:\state\lexicon `
  --scout-storage-root C:\state\scout `
  --receipts-storage-root C:\state\receipts `
  --allow-root C:\dev `
  --artifacts-dir .\artifacts\2026-08-09 `
  --context-manifest-file .\context.json `
  collect
```

It writes `lexicon-snapshot.json`, `scout-report.json`, and
`receipts-list.json` to `--artifacts-dir`, then prints the bounded
`inspector.ai-context.v1` result. Add `--mode snapshot` to print the full
`inspector.snapshot.v1` instead. The source-run command requires Go; users
packaging the applications may use equivalent application binaries and hand
the resulting JSON files directly to `inspector`.

An AI host can read the output directly:

```powershell
# One-shot, bounded context for any AI host.
go run ./cmd/inspector --ai-context --lexicon-snapshot-file .\artifacts\2026-08-09\lexicon-snapshot.json `
  --scout-report-file .\artifacts\2026-08-09\scout-report.json `
  --receipts-list-file .\artifacts\2026-08-09\receipts-list.json

# For a running local Inspector API:
# GET http://127.0.0.1:7341/v1/ai-context
```
