# Integration Test Task List

This document tracks integration-test coverage expansion for `dprint-plugin-shfmt`.

## P0 (High Priority)

- [x] Add a `format` success case that verifies expected output for an unformatted script.
- [x] Add a `no change` case where pre-formatted input is returned unchanged.
- [x] Add a parse-error case and assert the parser error text is surfaced.
- [x] Add variant-detection cases:
  - [x] `.sh` path with bash-only syntax fails (POSIX parsing).
  - [x] `.bash` path with bash-only syntax succeeds.
- [x] Add a shebang-precedence case where `#!/usr/bin/env bash` on a `.sh` path succeeds.
- [x] Add a global-override case where top-level `indentWidth`/`useTabs` override plugin-level values.
- [x] Add a comment/shebang preservation case to avoid dropping important script metadata.

## P1 (Medium Priority)

- [ ] Add option coverage for `useTabs`.
- [ ] Add option coverage for `binaryNextLine`.
- [ ] Add option coverage for `switchCaseIndent`.
- [ ] Add option coverage for `spaceRedirects`.
- [ ] Add option coverage for `funcNextLine`.
- [ ] Add option coverage for `minify`.

## P2 (Low Priority)

- [ ] Add configuration-type error coverage (for example `funcNextLine: "invalid"`).
- [ ] Add unknown-property diagnostic coverage.
- [ ] Add optional stress-style coverage for repeated invocations with the same cache directory.
