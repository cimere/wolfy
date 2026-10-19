# Wolfy

A WhatsApp assistant for a volleyball team's schedules, venues, and FAQs. The application uses **Go + bbolt** and builds with **CGO disabled**. It needs no database server or C compiler.

The repository implements the message flow and includes a local demo and tests with simulated integrations. A live deployment requires Meta, Google, and OpenAI accounts, official team data, and an HTTPS endpoint. No credentials or real user data are included.

## Run locally

Install Go **1.27.2** (the version in `.go-version`), `make`, and Python 3 to send the demo message.

```sh
go mod download
make check
make demo
```

In a second terminal:

```sh
python3 scripts/demo-webhook.py 'When do we train?'
curl -fsS http://127.0.0.1:8080/healthz
```

The demo authorizes only the **synthetic** ID `399000000001`, uses demo credentials, and returns a fixed response. It does not contact Meta, Sheets, or OpenAI. The `reply_accepted` log confirms processing; message contents are not printed. Stop the process with Ctrl+C. The local database is `data/demo.db`, which Git ignores.

## Checks

```sh
make check
make linux
make audit
make smoke
```

`make check` runs `gofmt` verification, `go vet`, tests, and a build with `CGO_ENABLED=0`. Tests use temporary files and simulated HTTP transports; after modules are downloaded, they need no credentials, Internet access, or listening ports. The demo requires a local port.

`make audit` runs the Makefile-pinned version of `govulncheck` and queries the public vulnerability database. A network error does not count as a clean scan. The race detector is not part of required checks because it requires CGO.

`make smoke` starts a temporary demo, submits a synthetic webhook, and checks maintenance commands. It requires Bash, curl, Python 3, and an available port 8080; it stops the process and removes test data. The GitHub workflow runs these checks weekly as well. Dependabot proposes module and action updates. Go and scanner versions require explicit updates. See [local validation status](docs/validation.md).

To inspect external modules actually imported by the application:

```sh
CGO_ENABLED=0 go list -deps -f '{{if .Module}}{{.Module.Path}}{{end}}' ./cmd/wolfy | sort -u
```

The module has two direct dependencies: bbolt and OAuth2. The full module graph also includes dependencies' tools and tests; not every module shown by `go list -m all` is linked into the executable. See [architecture](docs/architecture.md).

## Live configuration

1. Prepare the Google document using the [spreadsheet layout](docs/sheets.md).
2. Create a Google service account with **viewer** access to that document only, and enable the Sheets API.
3. Configure the Meta app and test number. Choose a supported Graph API version shown in the Meta dashboard.
4. Copy `.env.example` to `.env`, fill in the values, and protect the file with `chmod 600 .env`. Set `WOLFY_DB` to a writable local path during testing.
5. Load the environment and start the service:

```sh
set -a
. ./.env
set +a
make build
./bin/wolfy serve
```

For server setup, follow the [operations guide](docs/operations.md). The application listens only on loopback; Caddy exposes `/webhook` over HTTPS. `/healthz` is local and only confirms that the HTTP process responds.

## Behavior and limits

- One adult team, up to 30 active numbers, private text chats. The bot does not read WhatsApp groups or manage attendance and reminders.
- The calendar contains explicit events; Wolfy never writes to Sheets. It rereads the data for each question.
- Memory contains at most the last 12 messages and expires within 7 days. The Italian command `cancella memoria` deletes it from the active archive; `privacy` explains the basic processing and gives the human contact.
- If information is missing or invalid, Wolfy asks a clarifying question or gives the human contact. It sends no private information if access cannot be verified.
- Initial AI budget: USD 2 per UTC month. Accounting uses nano-USD, reserves a conservative amount before requests, and reconciles it against provider-reported token usage. Unknown outcomes keep the reservation.
- Default limit: 20 questions per user per UTC day; further questions are silently ignored. Privacy and deletion commands remain available.
- Wolfy does not send a reply more than 23 hours after the original message. It does not automatically repeat a send with an uncertain outcome.
- AI answers and resistance to malicious prompts require evaluation in a pilot. Deterministic tests verify software behavior but do not guarantee model accuracy.

The EUR 10/month figure is an operating target, not a cap on provider invoices. Check current prices, taxes, domain, hosting, and phone costs before launch.

## Project tracking

- [Assumptions](docs/assumptions.md): unverified planning and design claims, their owners, and how to validate them.
- [Blockers](docs/blockers.md): obstacles to validation or live activation, with next actions and completion conditions.
- [Validation](docs/validation.md): results of executed checks and checks still pending.
