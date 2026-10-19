# Implementation validation

Last local run: **October 19, 2026**, Go `1.27.2`, macOS arm64, `CGO_ENABLED=0`.

## Checks that passed

- `make check`: formatting, `go vet`, automated tests, and native build.
- `make linux`: Linux amd64 executable; `file` confirmed it is statically linked.
- `go mod verify`: checksums for downloaded modules verified.
- `go list -deps`: no package uses CGO; four external modules are imported by the application (bbolt, OAuth2, x/sys, and compute/metadata).
- `bash -n scripts/smoke-demo.sh`: script syntax is valid.

Tests include the signed webhook → Sheets → OpenAI → Meta flow with simulated HTTP transports. They cover deduplication, access revocation, memory isolation and deletion, calendar updates, outages, usage limits, uncertain outcomes, restarts, backups, and restores. Restore tests also cover retried webhooks whose IDs are missing from the backup. A month-end test protects the previous month's usage counter.

## Checks still pending

Audit and demo failures below were reproduced on October 19. Track owners, next actions, and resolutions in [the blocker register](blockers.md).

- **Vulnerability scan (B-001):** no result recorded; scanner download failed because of session DNS.
- **Local port demo (B-002):** no successful `make smoke` run recorded; the sandbox denied local listening. Simulated integration tests do not use sockets.
- **GitHub CI (B-006):** workflow is configured but has not run. GitHub rejected the initial push on October 19 because the HTTPS OAuth credential lacks `workflow` scope.
- **Live activation (B-003 through B-005):** authenticated provider checks, VPS installation, and the pilot are pending. Follow acceptance checks in `operations.md`, including response accuracy, pricing, and provider configuration.

Passing software tests does not prove the absence of vulnerabilities or the semantic accuracy of model responses. Update this document with actual audit, smoke test, and pilot results.
