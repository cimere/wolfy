# Running and maintaining Wolfy

## Deployment plan

Deployment is manual. GitHub Actions checks the repository but does not deploy it or hold production credentials. No live deployment is recorded yet. Track progress in [blockers](blockers.md) and record executed checks in [validation](validation.md).

| Phase | Work | Completion condition |
| --- | --- | --- |
| 1. Validate the candidate | Run `make check`, `make audit`, `make smoke`, and the Linux build for the server architecture. Record the Git commit and binary checksum (`shasum -a 256 bin/wolfy-linux-amd64` for the default target). | All checks pass; audit findings are addressed. B-001 and B-002 can be closed. |
| 2. Prepare accounts and data | Configure the Meta test number and recipients, Google service account and private spreadsheet, OpenAI project, fallback contact, and participant privacy information. Keep secrets outside Git. | Required credentials and official data are ready; ownership and retention are assigned. |
| 3. Provision and install | Choose the VPS and domain within budget, configure DNS and Caddy, install the binary and systemd service, and protect credentials and database files as described below. | The process is healthy locally and the HTTPS callback is reachable and verified by Meta. |
| 4. Verify integrations and recovery | Send an authorized test message through all providers; test revoked access, failures, backup, and safe restore. | Real integrations and recovery work, with results recorded. B-003 and B-004 can be closed. |
| 5. Run the pilot and expand | Use a few test recipients and complete the pilot acceptance checks below. Review answer quality, latency, actual costs, and remaining assumptions. | The project owner records a launch decision. B-005 can be closed; activate the final number and onboard the team. |

Keep the previous executable and a verified backup for each update. If startup, provider checks, or authorized replies fail, stop expansion and follow the rollback procedure below. Changing the final phone number requires checking its Meta registration and updating the corresponding configuration.

## Prerequisites and activation

You need a European Linux VPS with systemd, a hostname whose DNS points to the server, Caddy for HTTPS, and Meta, Google, and OpenAI accounts. An entry-level OVH server is an initial candidate to compare against the budget. Before purchasing, check the final price, VAT, contract duration, IPv4, backups, and domain cost. This repository does not purchase or activate any service.

Start with the test number in the Meta dashboard and recipients permitted by the account. Subscribe to message events and set the callback URL to `https://HOSTNAME/webhook`; the verification token must match `META_VERIFY_TOKEN`. Generate it with `openssl rand -hex 32`. The app secret is the secret for the Meta app, not the access token. Use a supported Graph API version and credentials with the required permissions; document their expiry and rotation.

For Google, enable the Sheets API, create a service account, and share the document with it as a viewer. Store its JSON key outside Git. For OpenAI, use a dedicated project, verify model access and pricing before the pilot, and set provider spending alerts. Wolfy's internal limit does not control provider invoices or usage outside the application.

## Install on the VPS

Build on an up-to-date development machine:

```sh
make check
make audit
make linux
```

The default Linux build targets amd64. For arm64:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o bin/wolfy-linux-arm64 ./cmd/wolfy
```

Transfer the executable, `deploy/wolfy.service`, and `deploy/Caddyfile` to the VPS. Install Caddy using the official instructions for your distribution. On the VPS, as an administrator:

```sh
sudo useradd --system --home-dir /var/lib/wolfy --shell /usr/sbin/nologin wolfy
sudo install -d -o root -g root -m 0755 /opt/wolfy
sudo install -d -o root -g wolfy -m 0750 /etc/wolfy
sudo install -d -o wolfy -g wolfy -m 0700 /var/lib/wolfy
sudo install -o root -g root -m 0755 wolfy-linux-amd64 /opt/wolfy/wolfy
sudo install -o root -g root -m 0644 wolfy.service /etc/systemd/system/wolfy.service
```

Create `/etc/wolfy/wolfy.env` from `.env.example` with real values. Install the Google JSON key at `/etc/wolfy/google-service-account.json`. Both files should be owned by `root:wolfy` with mode `0640`. Systemd reads the environment variables from its service configuration; it does not load a `.env` file automatically.

Replace the example domain in the Caddyfile and install it as `/etc/caddy/Caddyfile`. DNS must resolve to the VPS and ports 80/443 must be reachable; port 8080 remains on loopback. The Caddyfile exposes only `/webhook` and has no access logging.

```sh
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl daemon-reload
sudo systemctl enable --now wolfy
sudo systemctl reload caddy
curl -fsS http://127.0.0.1:8080/healthz
sudo journalctl -u wolfy -n 30 --no-pager
```

`healthz` checks that the HTTP process is running, not that providers are reachable. The `reply_accepted` log means Meta accepted the message; it does not confirm delivery to the phone. Status webhooks are not processed as messages.

## Monitoring and limits

Watch for `authorization_unavailable`, `knowledge_unavailable`, `generation_unavailable`, `send_stopped`, `send_retry_scheduled`, and `daily_limit_reached`, as well as the service status. Errors do not include message text, phone numbers, tokens, or external response bodies.

The limit of 20 questions per user per day uses UTC. The Italian commands `privacy` and `cancella memoria` do not use AI. The USD 2 AI limit resets each UTC month; reservations can exceed actual cost and remain charged when usage cannot be confirmed.

Maintenance commands require the service to be stopped because bbolt allows only one process to own the database. To read the counter:

```sh
sudo systemctl stop wolfy
sudo -u wolfy env WOLFY_DB=/var/lib/wolfy/wolfy.db /opt/wolfy/wolfy status
sudo systemctl start wolfy
```

One USD is 1,000,000,000 nano-USD. Do not raise the limit to hide accounting errors. If the model changes, set explicit prices and verify that the request parameters remain compatible.

## Backups

Use the dedicated command: it cleans up old records, checks database integrity, and creates a compacted copy. Do not copy the file while the process is modifying it. A short service interruption is acceptable during the pilot; Meta can retry webhooks.

```sh
sudo systemctl stop wolfy
sudo -u wolfy env WOLFY_DB=/var/lib/wolfy/wolfy.db /opt/wolfy/wolfy backup /var/lib/wolfy/backups/backup-2026-10-19.db
sudo systemctl start wolfy
```

Use the current date in the filename; an existing file is never overwritten. Check the command's exit status and restart the service even if the backup fails. Keep a separate encrypted copy with restricted access for no more than 7 days; account for any storage cost in the budget. Backups on the same disk do not protect against VPS loss.

Provider backups may have a different retention period. Set it consistently and inform users. Deleting active memory does not delete existing copies. Active database files may retain data in freed pages; periodically replace them with compacted copies during maintenance. Do not claim certified physical erasure.

## Restore

1. Stop Wolfy and preserve the current database in a protected location for diagnosis, within the retention period.
2. Install the chosen copy as `/var/lib/wolfy/wolfy.db`, owned by `wolfy:wolfy` with mode `0600`.
3. Run `check`, `cleanup`, and **`restore-safe`** before starting the service:

```sh
sudo -u wolfy env WOLFY_DB=/var/lib/wolfy/wolfy.db /opt/wolfy/wolfy check
sudo -u wolfy env WOLFY_DB=/var/lib/wolfy/wolfy.db /opt/wolfy/wolfy cleanup
sudo -u wolfy env WOLFY_DB=/var/lib/wolfy/wolfy.db /opt/wolfy/wolfy restore-safe
sudo systemctl start wolfy
```

`restore-safe` deletes restored memory, discards old outgoing messages, and pauses AI. It also ignores retried webhooks timestamped at or before the restore time, because a backup may not contain their IDs. A snapshot cannot know about later deletions, expenses, or sends. Users can ask again in a new message; while AI is paused, they receive the human contact. Keep the server clock synchronized.

Reconcile this month's costs in the dedicated OpenAI project, including usage after the backup and any uncertain calls. With the service stopped, run `resume-ai AMOUNT_IN_NANO_USD` with the conservative total. The command does not lower the existing counter. Restart the service. Do not use `0` to bypass the limit.

## Updates and rollback

Before an update, run checks and the audit, make a backup, and keep the previous executable. Stop the service, install the new binary, and restart it. Check health and send an authorized test question. Restore the previous binary if needed; if the data format changed, follow the safe restore procedure as well. The current schema version is `1`; an unknown schema prevents startup.

Regularly update Go, modules, the operating system, and Caddy. Record versions and check results. Limit operational log retention, for example to 7 days; do not add HTTP body logging to diagnose issues.

## Pilot acceptance

Using the Meta test number and a few authorized users, check real questions, follow-ups such as “and what time?”, changed and cancelled calendar events, missing FAQs, a revoked number, memory deletion, and a zero budget. Simulate Sheets and OpenAI outages and test a restore before expanding to the whole team.

Manually evaluate ambiguous and off-topic questions and malicious prompts. Repository tests cannot certify semantic accuracy of model responses. Record actual usage and observed latency; activate the final number only after the pilot.
