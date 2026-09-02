# ron1n changelog

## `0.0.1zoro` — 2026-09-02

This release replaces ron1n's Arch-only Bash/Python runtime with native Go binaries for Linux, Windows, and macOS on amd64 and arm64.

### Runtime

- Replaced the Python static server with a manifest-allowlisted Go host.
- Preserved the pinned firmware-9.00 PSFree -> Lapse -> AIO patch -> GoldHEN routes and bytes.
- Added exact `GET`, `HEAD`, conditional, ETag, cache-manifest MIME, and single-range behavior.
- Added completion-aware telemetry that distinguishes requests, partial transfers, completed transfers, and unknown console execution.
- Added bounded/rotating state and removed automatic ARP/OUI scanning, firewall mutation, and package installation.

### Different-network delivery

- Added the self-hostable `ron1n-relay` binary.
- Added outbound authenticated host polling so the host and console do not need the same LAN, inbound home ports, or a public home address.
- Added expiring/revocable capability URLs, host/session/request/byte limits, and an allowlisted `GET`/`HEAD`-only protocol.
- Added printed session management IDs and `ron1n relay revoke --session ID` for authenticated explicit revocation.
- Kept remote shells, command execution, arbitrary URL proxying, directory browsing, and uploads outside the protocol.

### Integrity and platform support

- Added pinned GitHub content import, safe archive extraction, deterministic manifests, Ed25519 signing, and per-request SHA-256 checks.
- Added native systemd-user, Windows Scheduled Task, and macOS LaunchAgent integrations.
- Added Bash and PowerShell bootstrap installers that verify 12 release binaries against same-origin `SHA256SUMS`.
- Added unit, race, relay integration, real upstream import, host compatibility, and cross-build verification.

The Ed25519 key in this release signs imported content manifests, not ron1n application binaries. `0.0.1zoro` does not include an application self-updater, independently signed application releases, or automatic application rollback; those remain future work.

PSFree, Lapse, AIO patches, GoldHEN, fonts, and other imported artifacts remain separately attributed upstream components. See `docs/UPSTREAM_AUDIT.md` and `docs/ECOSYSTEM_AUDIT.md`.
