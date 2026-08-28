# Inspector Desktop

Inspector Desktop is the portable native window for the Inspector read-only
local API. It embeds the UI in the application window; it does not launch or
depend on an external browser, a service, global configuration, or a harness.

The desktop client only sends `GET` requests to a loopback Inspector API. It
does not own or mutate Lexicon, Scout, Pith, or Receipts data.

## Run

Start the API in one terminal:

```text
go run ./cmd/inspector --snapshot-file ./sample-snapshot.json
```

Build/run the native client with a Rust toolchain and the platform webview
prerequisite:

```text
cargo run --manifest-path desktop/Cargo.toml -- --api http://127.0.0.1:7341
```

`--api` is optional and defaults to `http://127.0.0.1:7341`. Only `localhost`,
`127.0.0.1`, and `[::1]` endpoints are accepted.

The first build downloads the Tauri desktop framework if it is not already in
the local Cargo cache. That is a build dependency, not an application install;
the resulting executable is portable with its bundle/runtime requirements for
the target OS.
