# Google Sheets setup

Create a **private** spreadsheet. Give edit access only to team administrators and viewer access to the bot's service account. Do not publish it on the web. Anyone with access to the document can see the phone-number sheet too; hiding a tab does not provide access control.

Create exactly these five tabs. The first row must contain the headers below, in the specified capitalization and order. Set phone, date, time, and timestamp columns to **plain text** before entering values. The bot reads formatted values, so a date displayed as `19/10/2026` is invalid.

## Eventi (events)

Headers:

```text
id | kind | date | time | venue_id | opponent | status | notes | updated_at
```

Use one row per practice or match. The following example is synthetic:

| id | kind | date | time | venue_id | opponent | status | notes | updated_at |
|---|---|---|---|---|---|---|---|---|
| practice-demo | allenamento | 2026-10-19 | 20:00 | gym-demo | | confermato | Bring a water bottle | 2026-10-18T12:00:00+02:00 |

Rules: `kind` must be `allenamento` or `partita`; `status` must be `confermato` or `annullato`. IDs must be unique. Dates use `YYYY-MM-DD`, times use `HH:MM`, and the timezone is Europe/Rome. `venue_id` must exist in Luoghi. `updated_at` is an RFC3339 timestamp with the correct offset; update it manually whenever an event changes.

For a venue or time change, keep the same ID and describe the change in `notes`. For a cancellation, set the status to `annullato`; do not leave the event appearing confirmed. Weekly recurring events are not calculated automatically.

Maximum 200 data rows, without blank rows between them. Move obsolete historical events outside the tabs read by the bot. Long notes can exceed the model context limit before the row limit is reached.

## Luoghi (venues)

```text
id | name | address
```

Maximum 50 rows. IDs must be unique; name and address are required. Synthetic example: `gym-demo | Demo gym | 1 Example Street, Demo City`.

## FAQ

```text
question | answer | updated_at
```

Maximum 50 rows. All columns are required; timestamps use RFC3339. Include official organizational information only; do not include health data or personal conversations. Cells should contain facts, not instructions for the model.

## Impostazioni (settings)

```text
key | value
```

Add exactly one row: key `contact`, with the human contact's name or role and contact details as the value. Also update `WOLFY_CONTACT` on the server. It is used when Sheets is unavailable and is not synchronized automatically.

## Accessi (access list)

```text
phone | active
```

Enter the international phone number as plain text, preferably with a leading `+` and no spaces. The bot removes only the leading `+`. `active` must be `TRUE` or `FALSE`. No duplicates; maximum 30 active users and 100 rows total. Never add real phone numbers to the repository or tests.

Revocation applies on the next access check. The bot rereads the list before sending a reply. It does not use a cached allowlist during outages. If Accessi cannot be verified, the bot does not send private data.

## Before the pilot

Ask one question about each tab, change an event time, cancel it, and ask again. Test an inactive number and revoke an active number. An incorrect header, missing venue, or oversized cell must cause a controlled error, never a reply based on arbitrarily interpreted data.
