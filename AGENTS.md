# Repository Guidelines

## Project Structure & Module Organization

- `README.md`: Project entry point (currently minimal).
- `ref/`: Reference documents for plugin/wasm development (e.g., `ref/spec.md`, `ref/wasm-plugin-development.md`).
- `.devcontainer/`: Development container setup.
- `mise.toml`: Toolchain versions (Go, TinyGo).

Source code and tests are not present yet. When they are added, keep them in conventional locations (for example, `src/` or package-root Go modules, and `*_test.go` alongside code).

## Build, Test, and Development Commands

- `mise install`: Install the pinned toolchain defined in `mise.toml`.
- `go test ./...`: Run Go tests (once Go code and tests exist).
- `tinygo build ...`: Build TinyGo targets when wasm/plugin sources are added.

If you add project-specific scripts, document them here and keep the commands stable.

## Coding Style & Naming Conventions

- Use standard Go formatting via `gofmt` for Go sources.
- For shell scripts, format with `shfmt` and keep POSIX-compatible shells unless there’s a documented need for `bash`.
- Prefer clear, descriptive names; keep filenames lowercase with hyphens for docs (example: `ref/wasm-plugin-development.md`).

## Testing Guidelines

- No test framework is configured yet.
- When introducing tests, use Go’s standard `testing` package and the `*_test.go` naming convention.
- Keep unit tests colocated with the code they cover.

## Commit & Pull Request Guidelines

- Commit messages in history are short, imperative sentences with no prefix (example: “Add initial Dockerfile and setup scripts for development environment”).
- Write commit messages in English.
- Keep commits focused and descriptive.
- PRs should include a clear summary, rationale, and any relevant references (issues, specs, or screenshots for behavioral changes).

## Configuration & Tooling Notes

- Update `mise.toml` when bumping Go or TinyGo versions.
- Keep reference docs in `ref/` up to date with any implementation decisions.
- Write all project documentation in English.
- If sandbox or network restrictions block a required command, request temporary user approval and rerun with escalated permissions.
