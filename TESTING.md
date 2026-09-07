# ron1n `0.0.1zoro` verification record

Last updated: 2026-09-07

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

The local build verified the 12 binary names, their exact `0.0.1zoro` version output, and the generated `SHA256SUMS`. GitHub publication is also verified: `0.0.1zoro`, `0.0.1zoro-r1`, and `0.0.1zoro-r2` are immutable public releases with 13 uploaded assets each. The follow-up tag names are distribution revisions; the binaries continue to print exactly `0.0.1zoro`.

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
irm 'https://raw.githubusercontent.com/0xb0rn3/ron1n/acd7acb3f62ce099d1c792b993b8145de8240f0a/install.ps1' | iex
irm 'https://raw.githubusercontent.com/0xb0rn3/ron1n/d9be416c4c6d44e054ae60ac0f29ba688a412e17/scripts/windows-vm-smoke.ps1' | iex
```

- [x] Execute the original tag-pinned README `irm .../install.ps1 | iex` path from GitHub: failed before download because Windows PowerShell 5.1 lacks `RuntimeInformation.OSArchitecture`.
- [x] Execute the exact commit-pinned compatibility installer from the current README; checksums verified and `ron1n version` returned `0.0.1zoro`. Fix commit: `d4a8d5913768735ea75683876e78c4e62900d6ad`.
- [x] First full smoke attempt passed version, pinned import, NTFS signing, local health/MIME/hash/range, relay provisioning, and agent startup; its diagnostics helper then called `.Trim()` on an empty log. Harness-only fix: `d9be416c4c6d44e054ae60ac0f29ba688a412e17`.
- [x] The fixed full smoke completed: release checksum/version, pinned NTFS import/sign/verify, loopback health, cache MIME, full GoldHEN hash, 32-byte range, native relay, outbound agent, capability fetch with identical bytes, explicit session revocation to 404, and cleanup all passed.
- [x] `0.0.1zoro-r1` verified corrected Scheduled Task XML quoting and background restart/update, then exposed that task deletion did not end the already-running host.
- [x] Exact `0.0.1zoro-r2` installer from commit `acd7acb3f62ce099d1c792b993b8145de8240f0a` printed `Installed ron1n 0.0.1zoro from release 0.0.1zoro-r2` and `Release checksums verified`.
- [x] Elevated `ron1n install --service` imported pinned commit `368d82aa40d3017c220757ce315761adb5f06678`, installed autostart, and exposed health `status=ok`, version `0.0.1zoro`, bundle `284fae686d9ce20c5b68095614586750c32d4c9453c53f18ea6abc67cb1ca726`.
- [x] `ron1n restart` reported success and the same health/bundle identity returned after process replacement.
- [x] `ron1n update` activated the same pinned upstream revision, restarted the host, and retained the verified bundle identity.
- [x] `ron1n uninstall` ended the running task before deletion; host health became unreachable. A second uninstall also succeeded and preserved content, state, configuration, and signing keys.
- [x] `Get-ExecutionPolicy -List` reported `Undefined` for MachinePolicy, UserPolicy, Process, CurrentUser, and LocalMachine after testing.

Screenshot evidence from the final lifecycle run is retained locally as `/tmp/win10-r2-installer-8.ppm`, `/tmp/win10-r2-restart.ppm`, `/tmp/win10-before-uninstall.ppm`, `/tmp/win10-r2-uninstall.ppm`, `/tmp/win10-r2-uninstall-idempotent.ppm`, and `/tmp/win10-r2-policy.ppm`. These files contain no capability token or signing secret and are not release assets.

## Hardware-only gate

- [ ] PS4 firmware 9.00: first Application Cache install.
- [ ] Repeated offline/bookmark flow.
- [ ] PSFree and Lapse progression.
- [ ] AIO patch and GoldHEN binary requests.
- [ ] Console-side confirmation of successful execution.

HTTP tests cannot satisfy the last item. ron1n must continue to say transferred/served, not executed, until the console itself proves execution.
