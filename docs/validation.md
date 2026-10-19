# Implementation validation

Last local run: **October 19, 2026**, Go `1.27.2`, macOS arm64, `CGO_ENABLED=0`.

## Checks that passed

- `make check`: formatting, `go vet`, automated tests, and native build.
- `make linux`: Linux amd64 executable; `file` confirmed it is statically linked.
- `go mod verify`: checksums for downloaded modules verified.
- `go list -deps`: no package uses CGO; four external modules are imported by the application (bbolt, OAuth2, x/sys, and compute/metadata).
- `bash -n scripts/smoke-demo.sh`: script syntax is valid.

Tests include the signed webhook → Sheets → OpenAI → Meta flow with simulated HTTP transports. They cover deduplication, access revocation, memory isolation and deletion, calendar updates, outages, usage limits, uncertain outcomes, restarts, backups, and restores. Restore tests also cover retried webhooks whose IDs are missing from the backup. A month-end test protects the previous month's usage counter.

## GitHub CI

On **October 19, 2026**, [Checks run 39304828440](https://github.com/cimere/wolfy/actions/runs/39304828440) passed on commit `371d7d9`, using Ubuntu 24.04 and `CGO_ENABLED=0`. It verified module manifests and checksums, formatting, vet, tests, native and Linux builds, the absence of imported CGO files, `make smoke`, and the pinned `make audit` vulnerability scan.

SSH publication resolved B-006. The completed CI audit and demo resolved B-001 and B-002. The local session still cannot resolve the scanner download host or bind port 8080; those failures were reproduced on October 19, but the corresponding checks completed in CI.

## Checks still pending

**Live activation (B-003 through B-005):** authenticated provider checks, VPS installation, and the pilot are pending. Follow acceptance checks in `operations.md`, including response accuracy, pricing, and provider configuration. Track owners and next actions in [the blocker register](blockers.md).

Passing software tests does not prove the absence of vulnerabilities or the semantic accuracy of model responses. Update this document with actual audit, smoke test, and pilot results.
