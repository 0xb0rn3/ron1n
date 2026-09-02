# ron1n `0.0.1zoro` verification record

Last updated: 2026-09-02

## Automated Go gates

- `go test ./...`: passing.
- `go test -race ./...`: passing.
- `go vet ./...`: passing.
- `bash -n ron1n install.sh scripts/build-release.sh`: passing.
- `make release VERSION=0.0.1zoro`: passing; all generated assets pass `sha256sum -c SHA256SUMS`.
- End-to-end relay test: passing with four outbound agent workers, capability session, exact GoldHEN body, range request, expiry, revocation, authentication failure, and listener TLS policy.
- Host completion tests: passing for full GET, HEAD, 404, single range, private status, traversal, and simulated client disconnect.
- Import tests: passing for required files, SHA/signature, archive limits, and symlink rejection.
- Platform path tests: passing for Linux XDG, Windows known folders, and macOS Library paths.

## Native build matrix

The following binaries cross-compiled with `CGO_ENABLED=0`:

- `ron1n`: Linux amd64/arm64, Windows amd64/arm64, macOS amd64/arm64.
- `ron1n-relay`: Linux amd64/arm64, Windows amd64/arm64, macOS amd64/arm64.

## Application release boundary

The local build verified the 12 binary names, their exact `0.0.1zoro` version output, and the generated `SHA256SUMS`. It did not prove GitHub publication: the exact tag, release, and tagged raw installer URLs must exist before the copy-paste install paths can pass.

The Bash and PowerShell installers compare both downloaded binaries with `SHA256SUMS`, but all three are fetched from the same tagged GitHub trust domain. This is checksum verification, not an independent application signature. The Ed25519 content key signs imported content manifests only. Application self-update, independently signed application releases, and automatic application rollback are not implemented in `0.0.1zoro`.

Post-publication checks are defined in `docs/RELEASE.md`.

## Real pinned upstream integration

Source: `kmeps4/PSFree` commit `368d82aa40d3017c220757ce315761adb5f06678`.

Validated on the Linux build host with an isolated XDG root:

- GitHub ref resolved to the exact 40-character commit.
- GitHub PAX tar metadata was ignored while all content paths remained single-root and traversal-safe.
- Import completed into an immutable revision directory.
- A local Ed25519 key signed the deterministic manifest.
- Every manifest file was size/SHA-256 verified before server start and again on request.
- `/_ron1n/health` returned version `0.0.1zoro` and the active bundle ID.
- `psfree_lapse.cache` returned `Content-Type: text/cache-manifest; charset=utf-8`.
- Full `goldhen.bin` returned observed SHA-256 `c6329401d1810e16c84e6474ac30977dbdc951987c10cdb559370de7d59db0b0`.
- `Range: bytes=0-31` returned exactly 32 bytes.

The GitHub-generated archive SHA-256 observed in this run was `3d7e596086ba657fed53a1ec45d09685bc3049a8fa37dc849bd64ce7f9aba5b5`. GitHub tar archives are not guaranteed reproducible, so the durable identity is the resolved commit plus the signed per-file manifest, not this archive digest alone.

## Windows 10 QEMU/KVM acceptance

Target: libvirt domain `win10`, Windows 10, amd64, SPICE/QXL, NAT address observed as `192.168.122.148`.

The guest has no QEMU Guest Agent; testing is driven through its SPICE console with screenshot evidence. Required post-push gates:

```powershell
irm 'https://raw.githubusercontent.com/0xb0rn3/ron1n/0.0.1zoro/install.ps1' | iex
irm 'https://raw.githubusercontent.com/0xb0rn3/ron1n/0.0.1zoro/scripts/windows-vm-smoke.ps1' | iex
```

- [ ] Execute the exact tag-pinned README `irm .../install.ps1 | iex` path from GitHub.
- [ ] Confirm release checksum verification and native `ron1n version` output.
- [ ] Build/sign/verify the real pinned content manifest on NTFS.
- [ ] Start the native Windows host and validate loopback health, cache MIME, full hash, and range.
- [ ] Start native `ron1n-relay` plus outbound agent and complete a capability-URL fetch.
- [ ] Record the printed session ID, run `ron1n relay revoke --session ID`, and confirm the old capability URL is rejected.
- [ ] Confirm no permanent PowerShell execution-policy change.
- [ ] Capture results and update this file after the run.

## Hardware-only gate

- [ ] PS4 firmware 9.00: first Application Cache install.
- [ ] Repeated offline/bookmark flow.
- [ ] PSFree and Lapse progression.
- [ ] AIO patch and GoldHEN binary requests.
- [ ] Console-side confirmation of successful execution.

HTTP tests cannot satisfy the last item. ron1n must continue to say transferred/served, not executed, until the console itself proves execution.
