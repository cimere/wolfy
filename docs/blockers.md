# Blocker register

Updated: **October 19, 2026**.

Track obstacles to a specific next step, with an owner and an observable condition for clearing them. Use stable IDs (`B-001`, etc.) and statuses `Open`, `In progress`, or `Resolved`. Owners below are roles awaiting assignment. Keep resolution dates and evidence when closing entries.

The initial entries come from the implementation record dated **October 18, 2026**. B-001 and B-002 were reproduced on October 19; live activation still has no recorded results. They block validation or live activation as stated below; local development can continue.

## Open blockers

| ID | Status | Blocks | Recorded obstacle | Next action and completion evidence | Owner |
| --- | --- | --- | --- | --- | --- |
| B-001 | Open | Completion of the vulnerability review before live activation | The session could not download the pinned scanner because `proxy.golang.org` DNS resolution failed; no scan result is recorded. | Run `make audit` with Internet access. Record its date and output summary in [validation](validation.md); address findings before closing. | Technical maintainer |
| B-002 | Open | Validation of the running demo and maintenance commands together | `make smoke` could not bind `127.0.0.1:8080` in the sandbox (`operation not permitted`). | Run `make smoke` outside the sandbox with port 8080 free. Record the result in [validation](validation.md); fix any application failures. | Technical maintainer |
| B-003 | Open | Live provider integration checks | No successful authenticated Meta, Google Sheets, or OpenAI check is recorded. | Prepare the accounts, Meta test number and recipients, read-only Google service account, private spreadsheet, and OpenAI project. Configure secrets outside Git and complete an authorized message flow. See [Sheets setup](sheets.md) and [operations](operations.md). | Project owner and technical maintainer |
| B-004 | Open | Hosting the Meta callback and testing deployment/recovery | No VPS deployment or reachable HTTPS callback is recorded. | Select hosting within budget, configure DNS/Caddy/systemd, verify the callback, and rehearse backup and safe restore. Record outcomes and ongoing costs. See [operations](operations.md). | Technical maintainer |
| B-005 | Open | Expanding from test recipients to the team | No completed live pilot is recorded. | After B-001 through B-004 are cleared, assign the official-data owner and human contact, prepare the participant privacy information and retention arrangements, and perform the [pilot acceptance checks](operations.md#pilot-acceptance). Record model answer quality, failures, latency, usage, and the launch decision. | Project owner and pilot coordinator |
| B-006 | Open | Publishing the initial commit and running GitHub CI | On October 19, GitHub rejected the HTTPS push because the OAuth credential lacks `workflow` scope, required to create `.github/workflows/ci.yml`. No branch was published by that push. SSH access could not be verified because of session DNS restrictions. | Authenticate GitHub CLI with `workflow` scope and configure its Git credential helper; retry `git push -u origin main`. Close after GitHub confirms the branch exists, then inspect the CI run. | Repository owner and technical maintainer |

## Pending work without a recorded blocker

The public GitHub remote is configured at `https://github.com/cimere/wolfy`. The workflow has no recorded run yet; B-006 prevents the initial push. Record the CI result in [validation](validation.md) once publication succeeds. An unexecuted task becomes a blocker only when a concrete dependency or obstacle prevents the next step.

## Resolved blockers

None recorded yet. Move closed entries here with a resolution date and evidence. If an obstacle returns, reopen its existing ID. Link assumptions through their IDs in [the assumptions register](assumptions.md); record only verified progress and never include secrets.
