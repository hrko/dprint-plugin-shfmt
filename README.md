# dprint-plugin-shfmt

Shell script formatting plugin for dprint.

This uses the [`mvdan.cc/sh/v3`](https://github.com/mvdan/sh) parser and printer used by `shfmt`.

## Example config

This example enables the plugin, targets shell script files, and sets a few common formatting options.
`indentWidth` and `useTabs` are global dprint options, while settings under `shfmt` are plugin-specific.

```json
{
  "plugins": ["https://plugins.dprint.dev/hrko/shfmt-v0.0.1.wasm"],
  "includes": ["**/*.sh", "**/*.bash"],
  "indentWidth": 2,
  "useTabs": false,
  "shfmt": {
    "switchCaseIndent": true,
    "spaceRedirects": true,
    "funcNextLine": false
  }
}
```

## Configuration schema

See the schema for all available options and the latest canonical definitions.
- [schema.json](./schema.json)

## Development docs

For development documentation, see [AGENTS.md](./AGENTS.md).
