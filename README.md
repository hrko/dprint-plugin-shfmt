# dprint-plugin-shfmt

An implementation of `shfmt` (`mvdan.cc/sh/v3`) as a dprint Wasm plugin (Schema v4).

## Current implementation scope

- Schema v4 required exports
- Shared buffer implementation (`get_shared_bytes_ptr`, `clear_shared_bytes`)
- `register_config` / `release_config` / `get_config_diagnostics` / `get_resolved_config`
- `set_file_path` / `set_override_config` / `format` / `get_formatted_text` / `get_error_text`
- Formatting via `mvdan.cc/sh/v3/syntax`
- Dialect detection based on file extension and shebang
- Mapping of `indentWidth`, `useTabs`, `binaryNextLine`, `switchCaseIndent`, `spaceRedirects`, `funcNextLine`, and `minify`

## Setup

```bash
mise install
```

## Development commands

```bash
go generate ./...
mise run fmt
mise run lint
mise run test
mise run build-wasm
mise run release-check
mise run release-snapshot
```

`mise run build-wasm` generates `plugin.wasm`.
`mise run release-snapshot` generates release artifacts in `dist/` without publishing.
