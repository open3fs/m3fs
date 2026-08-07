# Repository Guidelines

## Project Structure & Module Organization

`cmd/m3fs/` contains the CLI entry point and subcommands. Reusable deployment logic lives under `pkg/`, grouped by service (`storage`, `mgmtd`, `fdb`, `grafana`, and others); shared task orchestration is in `pkg/task/`. Keep service configuration templates beside their package in `pkg/<service>/templates/`. Cross-package test helpers and mocks belong in `tests/`. Docker build contexts are under `dockerfile/`, utility scripts under `scripts/`, and `cluster.yml.sample` documents supported cluster configuration.

## Build, Test, and Development Commands

Use the Go version declared in `go.mod` (currently 1.23.7).

- `make build` compiles the static CLI to `bin/m3fs` and embeds version metadata.
- `./bin/m3fs --help` runs the locally built CLI and lists commands.
- `make test` runs all packages with a one-hour timeout.
- `go test ./pkg/config -run TestConfigSuite` runs a focused package test.
- `make checkfmt` reports Go files that differ from `gofmt` output.
- `make validate` installs the pinned `golangci-lint` version and runs the configured lint suite.

## Coding Style & Naming Conventions

Format Go code with `gofmt`; use tabs as produced by the formatter and let `goimports` organize imports. Follow standard Go naming: exported identifiers use `PascalCase`, local identifiers use `camelCase`, and package names remain short and lowercase. Preserve conventional initialisms such as `IP`, `URL`, `API`, `SSH`, and `JSON`; `.golangci.yml` enforces these and other checks. New Go files require the Apache 2.0 header; apply it with `./addlicense.sh <files>`.

## Testing Guidelines

Place tests next to implementation files as `*_test.go`. Name top-level tests `TestXxx`; existing suites use `testify/suite` with methods such as `TestValidateConfig`. Add regression coverage for behavioral changes and use helpers from `tests/base`, `tests/external`, or `tests/model` instead of duplicating mocks. No numeric coverage threshold is configured. The full CI test job expects PostgreSQL on port 5432, matching `.github/workflows/test.yml`.

## Commit & Pull Request Guidelines

Recent commits use brief, imperative subjects, often prefixed with `fix`, `add`, `support`, or `optimize` (for example, `fix storage service idempotency`). Keep each commit focused; PR numbers are typically appended during merge. Pull requests should explain the user-visible or operational impact, link relevant issues, list validation commands, and call out changes to `cluster.yml.sample`, templates, Docker images, or deployment behavior. Include terminal output or screenshots when CLI or architecture-rendering output changes.

## Security & Configuration

Do not commit real cluster passwords, SSH keys, registry credentials, or generated `.env` files. Prefer key-based SSH and use sanitized addresses and credentials in examples.
