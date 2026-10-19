# Assumptions register

Updated: **October 19, 2026**.

Record assumptions that implementation or planning depends on, including how to validate them and what changes if they prove false. Use stable IDs (`A-001`, etc.) and statuses `Unverified`, `Validated`, `Invalidated`, or `Retired`. Include dated evidence when changing a status; keep resolved entries for context. Owners below are roles to assign before the pilot.

## Confirmed baseline

The agreed scope is one adult team with up to 30 authorized users, private text chats, official data in read-only Sheets, Italian bot replies, and seven-day conversation memory. The implementation uses Go, bbolt, and one Linux server with CGO disabled. These are requirements and design decisions; see [architecture](architecture.md) for their implementation. The original brief is historical.

## Active assumptions

| ID | Assumption | Status | Owner | Validation and consequence if false |
| --- | --- | --- | --- | --- |
| A-001 | A single worker and fresh Sheets reads can handle the team's normal and peak traffic with acceptable response times. | Unverified | Technical maintainer | Measure throughput, queue delay, provider errors, and reply latency in the pilot. Agree acceptable latency with the team; revise processing or data access if necessary. |
| A-002 | Relevant official records and recent conversation history normally fit within the 24,000-byte request limit. | Unverified | Knowledge-base owner | Measure representative calendars, FAQs, and follow-up conversations. If they exceed the limit, reduce unnecessary records or design explicit data selection; oversized requests currently fall back to a human. |
| A-003 | Twelve messages within seven days provide enough context for useful follow-up questions. | Unverified | Pilot coordinator | Evaluate representative multi-turn conversations and deletion requests. Revisit the limit and privacy notice if more context is needed. |
| A-004 | The configured model, request parameters, and token prices are available and correct for the intended OpenAI account. | Unverified | Technical maintainer | Verify account access, provider documentation, pricing, and a live request before enabling AI. Update configuration and cost accounting if they differ. |
| A-005 | Expected usage fits the EUR 10/month operating target, excluding the phone number, with the initial USD 2 AI limit. | Unverified | Project owner | Record hosting, domain, backup, provider fees, taxes, exchange rates, and actual pilot usage. Adjust scope, budget, or hosting before committing to ongoing costs. |
| A-006 | Team administrators can keep the official Sheets data and access list accurate and maintain the fallback contact. | Unverified | Knowledge-base owner | Assign an administrator and rehearse event updates, cancellations, access revocation, and contact changes. Resolve ownership before onboarding players. |
| A-007 | One server and brief planned maintenance interruptions provide adequate availability for the team. | Unverified | Project owner | Agree service expectations and test backup/restore, including discarded old replies and AI cost reconciliation. Revisit hosting or recovery design if the interruption or data-loss window is unacceptable. |

## Maintenance

Add an entry when work relies on a new unverified claim. Link any resulting obstacle to [blockers](blockers.md). When evidence changes an assumption, record the date, result, and affected decision here; put architectural changes in [architecture](architecture.md) and executed check results in [validation](validation.md). Never store credentials or real conversations in this register.
