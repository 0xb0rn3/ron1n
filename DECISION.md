# ron1n coordination and decisions

Last updated: 2026-09-02

## Identity and ownership

- Project owner and Git author: `0xb0rn3 <154826956+0xb0rn3@users.noreply.github.com>`.
- Codex/lukk4n owns the `0.0.1zoro` Go rewrite until this claim is explicitly released.
- Chinam0k may review or take a separately claimed path, but must not edit an active lukk4n path without a written handoff here.
- Git remains the durable source of truth. AgentMemory is coordination transport, not authority.

## Active claim: LUKK4N-RON1N-001

Status: active

Scope:

- Replace the Bash/Python runtime with native Go host and relay binaries.
- Preserve the PSFree -> Lapse -> AIO patch -> GoldHEN browser/cache contract.
- Add opt-in delivery across different networks through an authenticated outbound agent and HTTPS relay.
- Add signed, allowlisted content manifests and verified imported-content update plumbing.
- Support native Linux and Windows binaries plus Bash and PowerShell automation.
- Add tests, build automation, ecosystem audit, operator documentation, and release metadata.

Owned paths: the complete repository until the first `0.0.1zoro` handoff commit.

## Product boundary

ron1n owns orchestration, importing, validation, HTTP delivery, activity reporting, remote relay transport, bootstrap installation, and imported-content updates. PSFree, Lapse, AIO patches, and GoldHEN are separately attributed upstream artifacts. They are not silently forked or described as ron1n code.

Release `0.0.1zoro` does not implement an application self-updater, independent application-release signatures, or automatic application rollback. Its bootstrap scripts verify release checksums inside the same tagged GitHub trust domain. Stronger release signing and rollback-aware self-update are future work and must not be represented as shipped.

## Non-LAN delivery decision

Remote delivery is explicit and off by default. A host agent makes outbound authenticated HTTPS requests to a self-hostable relay. A console browser receives an expiring capability URL and can fetch only files listed in the active verified bundle. The protocol has no shell, command execution, arbitrary URL proxy, filesystem browsing, or scanning operation.

Local LAN hosting remains available as an offline fallback and does not require the relay.

## Compatibility invariants

- Product version is exactly `0.0.1zoro`.
- Default local port remains `8080`.
- `/`, `/cache.html`, `psfree_lapse.cache`, ES module paths, `aio_patches.bin`, and `goldhen.bin` remain byte-preserving routes.
- Application Cache resources remain relative to the same URL prefix in both local and relay modes.
- The host supports concurrent requests because upstream initiates both binary fetches without waiting for the other.
- `GET`, `HEAD`, conditional requests, and a single byte range are supported.
- A payload is reported as transferred only after an allowlisted successful `GET` body completes. A `HEAD`, 404, interrupted response, partial range, or merely requested path is not a complete delivery.
- "Transferred" never claims kernel execution succeeded.
- PlayStation User-Agent and local neighbor data are observations, never authentication.

## Security invariants

- No public client-detail status endpoint.
- No directory listings, `.git` exposure, symlink escape, traversal, undeclared file, arbitrary upload, or remote execution path.
- Agent credentials and signing keys are stored separately from non-secret configuration and are never logged.
- Production remote agents require HTTPS. A plaintext exception exists only behind an explicit development flag.
- Relay sessions are high-entropy, expiring, revocable, request/byte bounded, and redacted in logs.
- Session creation prints a management ID separately from the secret capability URL; `ron1n relay revoke --session ID` performs authenticated explicit revocation.
- Imported content is staged, length/hash/signature checked, and atomically activated without overwriting the prior bundle.
- Application binaries are installed from the exact tagged GitHub release after same-origin `SHA256SUMS` verification; this is not an independent signature.
- Runtime installation never silently changes the firewall, disables unrelated services, or installs system packages.

## Release publication decision

- Publish the exact `0.0.1zoro` tag from the clean, pushed release commit.
- Build 12 native binaries from that clean tag and attach them with `SHA256SUMS` to the matching GitHub release.
- Pin copy-paste bootstrap URLs to an immutable tag or reviewed full commit, never mutable `main`. Windows `0.0.1zoro` uses compatibility-fix commit `d4a8d5913768735ea75683876e78c4e62900d6ad` because the original tagged script referenced a .NET property absent from the Windows 10 PowerShell 5.1 guest; release binaries remain tag assets.
- Never move a published tag or replace its assets; issue a new version instead.
- Follow `docs/RELEASE.md` and complete the post-publication Linux URL and Windows VM checks before declaring the release validated.

## Handoff rule

Before editing, reread this file and inspect `git status`. Preserve every dirty path. On handoff, record exact files, tests, known hardware gaps, commit, and push status in `BUILD_STATUS.md`, then save the same non-secret summary to AgentMemory.
