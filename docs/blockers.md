# Blocker register

Updated: **October 19, 2026**.

Track obstacles to a specific next step, with an owner and an observable condition for clearing them. Use stable IDs (`B-001`, etc.) and statuses `Open`, `In progress`, or `Resolved`. Owners below are roles awaiting assignment. Keep resolution dates and evidence when closing entries.

The initial entries come from the implementation record dated **October 18, 2026**. GitHub CI cleared B-001 and B-002 on October 19 after SSH publication cleared B-006. Live activation still has no recorded results; the remaining blockers concern provider setup, hosting, and the pilot.

## Open blockers

| ID | Status | Blocks | Recorded obstacle | Next action and completion evidence | Owner |
| --- | --- | --- | --- | --- | --- |
| B-003 | Open | Live provider integration checks | No successful authenticated Meta, Google Sheets, or OpenAI check is recorded. | Prepare the accounts, Meta test number and recipients, read-only Google service account, private spreadsheet, and OpenAI project. Configure secrets outside Git and complete an authorized message flow. See [Sheets setup](sheets.md) and [operations](operations.md). | Project owner and technical maintainer |
| B-004 | Open | Hosting the Meta callback and testing deployment/recovery | No VPS deployment or reachable HTTPS callback is recorded. | Select hosting within budget, configure DNS/Caddy/systemd, verify the callback, and rehearse backup and safe restore. Record outcomes and ongoing costs. See [operations](operations.md). | Technical maintainer |
| B-005 | Open | Expanding from test recipients to the team | No completed live pilot is recorded. | After B-001 through B-004 are cleared, assign the official-data owner and human contact, prepare the participant privacy information and retention arrangements, and perform the [pilot acceptance checks](operations.md#pilot-acceptance). Record model answer quality, failures, latency, usage, and the launch decision. | Project owner and pilot coordinator |

## Pending work without a recorded blocker

The public GitHub remote is configured at `https://github.com/cimere/wolfy`. The initial commits were published to `main`, and [the first Checks run](https://github.com/cimere/wolfy/actions/runs/39304828440) passed on October 19; see [validation](validation.md). An unexecuted task becomes a blocker only when a concrete dependency or obstacle prevents the next step.

## Resolved blockers

| ID | Status | Resolved | Evidence |
| --- | --- | --- | --- |
| B-001 | Resolved | October 19, 2026 | The pinned `make audit` step passed in [GitHub Checks](https://github.com/cimere/wolfy/actions/runs/39304828440) on commit `371d7d9`. Local scanner downloads remain restricted by session DNS; the check completed in CI. |
| B-002 | Resolved | October 19, 2026 | `make smoke` passed in the same GitHub run, exercising the local HTTP demo and offline maintenance commands on Ubuntu. The session's local bind restriction still applies. |
| B-006 | Resolved | October 19, 2026 | HTTPS rejected creation of the workflow because the OAuth credential lacked `workflow` scope. `git push git@github.com:cimere/wolfy.git main` succeeded using the existing SSH authentication. Commits `514af6a` and `371d7d9` are on GitHub; the Checks workflow started. HTTPS authorization for future workflow changes remains separate; use SSH if it still lacks the required scope. |

Move closed entries here with a resolution date and evidence. If an obstacle returns, reopen its existing ID. Link assumptions through their IDs in [the assumptions register](assumptions.md); record only verified progress and never include secrets.
