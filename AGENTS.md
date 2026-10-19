# Repository Guidelines

## Purpose & Architecture

Wolfy answers organizational questions for one adult volleyball team of up to 30 authorized users through private WhatsApp text chats. The confirmed implementation is Go with bbolt on a small Linux server, Meta Cloud API, OpenAI Responses API, and read-only Google Sheets. Read `docs/architecture.md`; `docs/project-brief.md` is the historical proposal.

## Project Structure

`cmd/wolfy/` contains the executable and maintenance commands. `internal/wolfy/` contains application logic and adjacent tests. `deploy/` contains systemd and Caddy configuration; `scripts/` contains demonstrations; `docs/` contains project documentation.

Maintain `docs/assumptions.md` and `docs/blockers.md` with stable IDs, status, owner, and validation or resolution evidence. Keep executed check results in `docs/validation.md`.

## Build, Test & Development

Use the Go version in `.go-version`. Application builds and tests must work with `CGO_ENABLED=0`: no C compiler, CGO, SQLite driver, or native application dependency.

- `go mod download`: fetch pinned modules.
- `make check`: check formatting, run vet and tests, and build.
- `make demo`: run local simulated integrations without provider credentials.
- `python3 scripts/demo-webhook.py`: submit a synthetic local webhook.
- `make linux`: build the Linux amd64 executable.
- `make audit`: run the pinned vulnerability scanner; requires network access.

## Style & Dependencies

Use `gofmt`, idiomatic Go names, and English identifiers. Keep user-facing bot messages clear and consistent in Italian; write project documentation in English. Prefer the standard library. Justify each added module, inspect transitive dependencies, and commit `go.mod` and `go.sum` together. Keep database transactions short; perform network calls outside them. Do not implement custom cryptography to avoid a maintained authentication library.

## Behavioral & Security Rules

Sheets is authoritative; player messages never modify official information. Keep conversation memory isolated by user and limited to seven days. Never send access lists to AI. Validate authorization before processing and sending. Ask for clarification or provide the configured human contact when facts are missing. Fail closed when authorization cannot be established.

Preserve durable deduplication, conservative handling of uncertain sends, and budget reservations. Never commit credentials, real numbers, databases, or transcripts. Avoid logging message contents or credential-bearing requests.

## Testing & Contributions

Use standard Go tests named `*_test.go` with synthetic data and mocked HTTP transports. Cover authorization, isolation, expiry, duplicates, updated calendars, outages, budget limits, crashes, and backup recovery. Keep live pilot verification separate; passing deterministic tests does not validate model answers.

Use focused imperative commits. Pull requests explain behavior, linked issues, validation, and configuration changes. Report unexecuted checks explicitly.
