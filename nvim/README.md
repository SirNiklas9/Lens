# Inspector for Neovim / Neovide

This is an optional, self-contained Inspector client. It renders the existing
read-only Inspector snapshot in a Neovim split or floating window. It does not
start Inspector, write source data, modify Neovim configuration, launch a
browser, or install anything globally.

## Use locally

Add this directory to Neovim's runtime path for one launch (or use your normal
plugin manager to point at this repository), then run either command:

```vim
:set rtp+=/path/to/inspector/nvim
:InspectorOpen --api=http://127.0.0.1:7341
:InspectorOpen --snapshot=/path/to/snapshot.json
```

The API form reads `GET /v1/snapshot` from a loopback address only. The
snapshot form reads an explicit JSON file. Either source may be unavailable;
the client renders an actionable unavailable state rather than failing the
editor.

Optional configuration belongs in *your* local Neovim setup; the client does
not create it:

```lua
require("inspector").setup({
  view = "side", -- "float" is the default
  api_url = "http://127.0.0.1:7341",
})
```

Commands:

- `:InspectorOpen [--api=URL|--snapshot=PATH]`
- `:InspectorRefresh`
- `:InspectorClose`
- `:LensLive [--api=URL|--snapshot=PATH]` opens the same view with the live
  review projection when the supplied Lens source includes `live` events.
- `:LensFollow` jumps to the latest coherent AI edit.
- `:LensReviewNext` and `:LensReviewPrevious` move through the changed-symbol
  review queue.
- `:LensReviewMark`, `:LensReviewAccept`, and `:LensReviewReject` emit an
  explicit durable decision through your configured `lens-review` command.

The Lens navigation commands are intentional user actions. They never start a
watcher or automatically steal your viewport.

When the target source file is openable, Lens places a transient, Lens-owned
extmark at the selected line (for example `AI editing applyLive`). It writes no
source text, does not imitate character-by-character typing, and clears only
its own marker on the next Lens navigation action.

Review markers use Neovim signs and end-of-line labels: `A+` new, `A~`
checking, `A!` failed, `A✓` passing, `U✓` reviewed, `OK` accepted, and `X`
rejected. They are Lens-owned decoration, not edits to source buffers.

Inside the Lens buffer, `]u` / `[u` select the next/previous review item;
`a`, `x`, and `v` accept, reject, or mark it reviewed. Normal buffers retain
their own mappings.

To enable decisions, opt in from your own Neovim configuration. Lens does not
guess executable, event, or Trace storage paths:

```lua
require("inspector").setup({
  review = {
    command = { "C:/AI-Deck/bin/lens-review.exe" },
    events_file = "C:/state/lens/events.jsonl",
    actor = "nicholas",
    scope = "project",
    trace = {
      binary = "C:/AI-Deck/bin/trace.exe",
      app = "C:/AI-Deck/apps/trace/application/pulp.app.toml",
      storage_root = "C:/state/trace",
    },
  },
})
```

The view shows active scope, context baseline/delta information, health and
freshness projections, Scry proposals, Splice status, Trace/test evidence,
and any legacy migration status supplied by the API. Inspector's source data
remains authoritative; this plugin only renders its snapshot.
