# Contributing to sssstore

## Development workflow

1. Run `make fmt test vet build` before opening a PR.
2. Keep changes scoped and include tests for behavior changes.
3. Update docs (`README.md`, `docs/*`) when APIs/config change.

## Commit and PR guidelines

- Use clear, imperative commit messages.
- PRs should include:
  - motivation,
  - implementation summary,
  - test evidence.

## Local commands

- `make test`
- `make vet`
- `make build`
- `go run ./cmd/sssstore init --config ./sssstore.json --data ./data`
- `go run ./cmd/sssstore server --config ./sssstore.json`
