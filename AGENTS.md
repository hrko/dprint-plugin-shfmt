# Repository Guidelines

## Project Structure & Module Organization
This repository implements a dprint Wasm plugin for `shfmt` in Go/TinyGo.

- Root package: plugin entrypoint and handlers (`main.go`, `handler_*.go`).
- `dprint/`: reusable runtime bridge, config resolver, and shared plugin types.
- `integration/`: end-to-end tests (`integration_test.go`) and fixtures in `integration/testdata/cases/<case>/`.
- `schema.json`: published plugin schema (generated).
- Generated files: `*_generated.go` and `schema.json` (regenerate; do not hand-edit).
- Build output: `plugin.wasm`; release artifacts: `dist/`.

## Build, Test, and Development Commands
Use `mise` to keep tool versions consistent.

- `mise install`: install pinned tools (Go, TinyGo, golangci-lint, dprint, goreleaser).
- `mise run generate`: run `go generate ./...` for boilerplate, config resolver, and schema outputs.
- `mise run lint-fix`: run `golangci-lint run --fix ./...` for auto-fix and formatting.
- `mise run fmt-dprint`: format Markdown/JSON/TOML/YAML files via dprint.
- `mise run lint`: run `golangci-lint` (includes `gofumpt`, `gci`, and enabled linters).
- `mise run test`: run unit tests (`go test ./...`).
- `mise run test-integration`: run integration tests with `-tags=integration`.
- `mise run build-wasm`: produce `plugin.wasm` with TinyGo.
- `mise run release-check`: validate `.goreleaser.yaml`.
- `mise run release-snapshot`: build local release artifacts without publishing.

## Coding Style & Naming Conventions
- Follow standard Go style; rely on formatters, not manual alignment.
- Keep files focused by responsibility (for example `handler_format.go`, `handler_config.go`).
- Test files must use `*_test.go`; prefer table-driven cases where practical.
- Never manually edit generated files; update generators/specs, then run `mise run generate`.

## Testing Guidelines
- Unit tests use Go’s `testing` package and should cover handler/config/runtime behavior.
- Integration cases are fixture-based: each case directory contains `config.json`, `input.sh`, and `expected.stdout`.
- No strict coverage threshold is enforced, but behavior changes should include tests at the appropriate level.

## Commit & Pull Request Guidelines
- Match existing commit style: short, imperative summaries (for example, `Split runtime internals into dedicated modules`).
- Keep commits scoped to one logical change.
- Before committing, confirm documentation files (for example `README.md` and `AGENTS.md`) reflect the latest project state.
- PRs should clearly describe what changed and why.
- Link issue(s) when applicable.
- Include test evidence (commands run, such as `mise run test` and `mise run test-integration`).
- Call out schema updates or release-impacting changes.

## Release Procedure
- Official releases are created by GitHub Actions when a tag matching `v*` is pushed (see `.github/workflows/release.yml`).
- Recommended flow:
  1. Run checks locally (`mise run lint`, `mise run test`, and optionally `mise run test-integration`).
  2. Create a version tag (for example, `git tag -a v0.0.1 -m "v0.0.1"`).
  3. Push the tag (`git push origin v0.0.1`).
- Do not rely on local `mise run release` for normal releases; CI provides `GITHUB_TOKEN` and publishes the release automatically.

## Documentation Language & Sandbox Constraints
- Write project documentation in English (README, guides, and in-repo reference docs).
- If a required command is blocked by sandbox or network restrictions, request temporary approval and rerun with escalation.
