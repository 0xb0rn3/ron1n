# ron1n build status

Last updated: 2026-09-07

Target release: `0.0.1zoro`

## Current state

- Legacy Bash/Python behavior audited.
- Current PSFree cache, module, Lapse, AIO patch, and GoldHEN request chain audited at upstream commit `368d82aa40d3017c220757ce315761adb5f06678`.
- Secure non-LAN relay architecture selected and recorded in `DECISION.md` and `ARCHITECTURE.md`.
- Go host, importer, state engine, relay/control plane, native CLI, ecosystem catalog, Linux/Windows/macOS integration, and verified installers implemented under `LUKK4N-RON1N-001`.
- Real pinned upstream integration passed for cache MIME, full GoldHEN SHA-256, and byte range. Exact evidence is in `TESTING.md`.
- Session creation now prints a management ID and the CLI exposes `ron1n relay revoke --session ID` for authenticated explicit revocation.
- Release trust is now stated precisely: imported content uses local Ed25519 signatures; application bootstrap uses same-origin GitHub release checksums. Application self-update, independent release signing, and automatic application rollback are future work.
- The original `0.0.1zoro` release and immutable Windows distribution revisions `0.0.1zoro-r1` and `0.0.1zoro-r2` are published with 13 assets each. Product version remains exactly `0.0.1zoro`.
- Windows 10 QEMU/KVM acceptance is complete. The exact GitHub `r2` installer checksum-verified both binaries, imported and signed the pinned PSFree bundle on NTFS, served the expected health/bundle identity, restarted, updated pinned content, stopped and removed the Scheduled Task, passed repeat uninstall, and left every permanent execution-policy scope undefined.
- The full Windows host/relay smoke separately passed exact cache MIME, GoldHEN SHA-256, range behavior, capability delivery, explicit revocation, cleanup, and policy preservation. Evidence and revision history are in `TESTING.md`.
- The operator-dashboard design contract is pushed in commit `479e62d`. Chinam0k owns only the fixture-backed `ui/prototype/**` handoff; lukk4n retains the loopback Go API and final integration.

## Release gates

- [x] Go host replaces the Python server without changing upstream bytes.
- [x] Linux, Windows, and macOS native CLI/relay binaries cross-compile for amd64/arm64.
- [x] Bash and PowerShell install/build paths pass syntax/runtime checks.
- [x] Local Application Cache and required route tests pass.
- [x] Remote relay integration works through an outbound-only host agent with no inbound host listener.
- [x] Expired/revoked/wrong-host sessions and unauthorized agents fail closed.
- [x] Manifest hash/signature, traversal, symlink, archive, and tampering tests pass.
- [x] `go test ./...`, race tests, vet, and all 12 release cross-builds pass.
- [x] Clean commit is tagged exactly `0.0.1zoro`; 12 binaries plus `SHA256SUMS` are attached. Immutable Windows revisions `r1` and `r2` preserve the product version and their installer/assets return HTTP 200 per `docs/RELEASE.md`.
- [ ] Hardware validation on a PS4 9.00 console is documented; transfer and execution are reported separately.
- [x] Implementation, release fixes, and UI handoff are authored as 0xb0rn3 and pushed to `origin/main`.

## Known legacy defects being removed

- `RON1N_RECENT_HTTP_SECONDS` is defined but never enforced.
- `HEAD`, 404, or interrupted GoldHEN requests can be labeled delivered before any successful body transfer.
- `aio_patches.bin`, required by the audited upstream page, is absent from the legacy validation list.
- A mutable upstream branch is pulled without a commit pin or content hash.
- The entire checkout, directory listings, symlinks, and a client-detail status endpoint are exposed on `0.0.0.0`.
- Firewall changes and privileged LAN scans are automatic and weakly checked.

## Hardware-only follow-up

Automated tests can prove byte, route, cache, range, and transport behavior. Only a real firmware-9.00 console can validate exploit reliability and successful kernel execution. That gate must not be represented as complete from HTTP telemetry alone.
