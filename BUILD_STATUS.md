# ron1n build status

Last updated: 2026-09-02

Target release: `0.0.1zoro`

## Current state

- Legacy Bash/Python behavior audited.
- Current PSFree cache, module, Lapse, AIO patch, and GoldHEN request chain audited at upstream commit `368d82aa40d3017c220757ce315761adb5f06678`.
- Secure non-LAN relay architecture selected and recorded in `DECISION.md` and `ARCHITECTURE.md`.
- Go host, importer, state engine, relay/control plane, native CLI, ecosystem catalog, Linux/Windows/macOS integration, and verified installers implemented under `LUKK4N-RON1N-001`.
- Real pinned upstream integration passed for cache MIME, full GoldHEN SHA-256, and byte range. Exact evidence is in `TESTING.md`.
- Session creation now prints a management ID and the CLI exposes `ron1n relay revoke --session ID` for authenticated explicit revocation.
- Release trust is now stated precisely: imported content uses local Ed25519 signatures; application bootstrap uses same-origin GitHub release checksums. Application self-update, independent release signing, and automatic application rollback are future work.
- Windows 10 QEMU/KVM acceptance is the remaining pre-handoff runtime gate; per owner instruction it begins only after GitHub push/release publication.

## Release gates

- [x] Go host replaces the Python server without changing upstream bytes.
- [x] Linux, Windows, and macOS native CLI/relay binaries cross-compile for amd64/arm64.
- [ ] Bash and PowerShell install/build paths pass syntax/runtime checks (Bash passed; Windows VM pending).
- [x] Local Application Cache and required route tests pass.
- [x] Remote relay integration works through an outbound-only host agent with no inbound host listener.
- [x] Expired/revoked/wrong-host sessions and unauthorized agents fail closed.
- [x] Manifest hash/signature, traversal, symlink, archive, and tampering tests pass.
- [x] `go test ./...`, race tests, vet, and all 12 release cross-builds pass.
- [ ] Clean commit is tagged exactly `0.0.1zoro`; 12 binaries plus `SHA256SUMS` are attached to the matching GitHub release and the tagged installer URLs return HTTP 200 per `docs/RELEASE.md`.
- [ ] Hardware validation on a PS4 9.00 console is documented; transfer and execution are reported separately.
- [ ] Commit is authored as 0xb0rn3 and pushed to `origin/main`.

## Known legacy defects being removed

- `RON1N_RECENT_HTTP_SECONDS` is defined but never enforced.
- `HEAD`, 404, or interrupted GoldHEN requests can be labeled delivered before any successful body transfer.
- `aio_patches.bin`, required by the audited upstream page, is absent from the legacy validation list.
- A mutable upstream branch is pulled without a commit pin or content hash.
- The entire checkout, directory listings, symlinks, and a client-detail status endpoint are exposed on `0.0.0.0`.
- Firewall changes and privileged LAN scans are automatic and weakly checked.

## Hardware-only follow-up

Automated tests can prove byte, route, cache, range, and transport behavior. Only a real firmware-9.00 console can validate exploit reliability and successful kernel execution. That gate must not be represented as complete from HTTP telemetry alone.
