# Architecture and confirmed decisions

This document supersedes the initial technology proposals: **Go, bbolt, and a Linux VPS**, with no C/CGO in the application. `project-brief.md` is retained as a historical proposal; n8n, TypeScript, Cloudflare, and SQLite are not part of the implementation.

Track unverified planning claims in [assumptions](assumptions.md) and obstacles to progress in [blockers](blockers.md). This document describes confirmed decisions and implemented behavior.

The GitHub repository is public, as confirmed on October 19, 2026. Keep provider credentials, real phone numbers, databases, and chat transcripts outside version control.

## Components

```text
Private WhatsApp chat → Meta → HTTPS webhook/Caddy → Go process
                                                  ├─ bbolt: queue, memory, deduplication, budget
                                                  ├─ Google Sheets: access list and official data
                                                  ├─ OpenAI Responses API
                                                  └─ Meta: text reply
```

A single process opens the bbolt file, which is protected by a lock. One worker processes the queue in receipt order per user. Transactions contain local database operations only; HTTP requests run outside them.

## Interfaces and boundaries

- `GET /webhook`: verifies Meta's subscription; compares the token in constant time.
- `POST /webhook`: verifies HMAC-SHA256 over the original body, limits it to 512 KiB, checks the configured recipient number, and persists messages atomically before returning 200. Status events do not produce replies.
- `GET /healthz`: local process check with no operational data or credentials.
- `Directory`: checks access and reads official data, kept separate in Go types.
- `Model`: generates text and reports token usage. `Messenger`: sends a reply and distinguishes accepted, definitively rejected, and uncertain outcomes.

HTTP clients have timeouts, response-size limits, and do not follow redirects. Google access uses a service account with the `spreadsheets.readonly` scope. The access list is never sent to the model.

## Queue and failures

The main states are `pending → preparing → ready → sending → sent`. Completion removes message text and recipient from the queue record; only accepted exchanges are added to memory.

After a restart, `preparing` returns to the queue, informational replies in `ready` are regenerated, and `sending` becomes `uncertain`. Exactly-once delivery cannot be guaranteed between the local database and Meta. The system favors avoiding a duplicate reply, even if a reply is lost after a crash.

Only Meta HTTP 429 responses are automatically retried, with at most three total send attempts. Timeouts, 5xx errors, and unreadable responses have uncertain outcomes. An informational retry rereads official data and regenerates the reply. No retry is sent more than 23 hours after the original message.

The queue holds up to 1,000 active jobs. When full, the webhook returns 503 so Meta can retry. Failed access checks get at most three attempts; the bot does not expose private data if it cannot verify authorization. If access is verified but official data is unavailable, it uses the human contact configured in the environment.

## Memory and data

Memory is isolated by phone number and limited to 12 messages within 7 days. Expired data is excluded from reads immediately and purged at startup and hourly. Current official data takes precedence over old replies. Chat messages cannot change Sheets.

Deletion from the active database is logical: bbolt may retain bytes in freed pages until compaction. Backups made by the dedicated command are compacted and omit those pages. Old backups must expire within 7 days; memory lifetime differs from backup retention and external service policies. After a restore, `restore-safe` clears memory and the old queue, and ignores webhook messages timestamped at or before the restore time, even when their IDs are absent from the snapshot.

The bbolt file is not encrypted by the application. Restrict access to the server; use directory mode 0700 and file mode 0600. Do not log phone numbers, message text, credentials, or provider request bodies. OpenAI requests set `store:false`; this does not change other providers' data retention policies.

## Costs and model

The initial model is `gpt-5.4-mini-2026-03-17`, with no additional reasoning. Another model requires explicit prices and compatibility with `reasoning.effort=none`. Requests include official information, the current time in Italy, and memory as untrusted data. Total context is limited to 24,000 bytes; if it exceeds the limit, the bot refers the user to the human contact instead of silently truncating the calendar.

The reservation assumes one token per UTF-8 input byte, 4,096 tokens of framing overhead, and the configured maximum output. This is a conservative bound, not measured tokenization. Usage is reconciled without cache discounts. If usage is unknown after an error or restart, the full reservation remains charged. Every AI retry reserves again.

The default limit is 2,000,000,000 nano-USD per UTC month. A restore suspends AI until an operator reconciles costs after the backup. Taxes, exchange rates, hosting, domains, and other services are outside this counter.

## Dependencies

Direct dependencies: `go.etcd.io/bbolt` for persistent transactions and `golang.org/x/oauth2` for maintained authentication. The program also imports `golang.org/x/sys` and `cloud.google.com/go/compute/metadata` transitively. Upstream test and CLI modules are not automatically linked into the bot.

Go, module versions, scanner, and CI actions are pinned and should be updated deliberately. `CGO_ENABLED=0` is required for builds and tests. The application uses no SQLite, web framework, ORM, or general-purpose provider SDK. Caddy and Linux are separate operational components that must also be maintained.

## Technical references

- [bbolt](https://github.com/etcd-io/bbolt)
- [Go OAuth2](https://pkg.go.dev/golang.org/x/oauth2)
- [OpenAI: text generation](https://developers.openai.com/api/docs/guides/text)
- [OpenAI: data controls](https://developers.openai.com/api/docs/guides/your-data)
- [Sheets API: usage limits](https://developers.google.com/workspace/sheets/api/limits)
- [WhatsApp: service window and pricing](https://whatsappbusiness.com/products/platform-pricing/)
