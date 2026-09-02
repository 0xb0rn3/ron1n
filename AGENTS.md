# ron1n agent guide

Read `DECISION.md`, `ARCHITECTURE.md`, `BUILD_STATUS.md`, `TESTING.md`, and `git status` before editing.

## Identities

- Owner and Git author: 0xb0rn3.
- Codex: lukk4n.
- Claude Code: chinam0k.

## Coordination

- Record a non-overlapping claim in `DECISION.md` before edits.
- Preserve every dirty path and commit only explicit owned paths.
- Git plus the Markdown record is authority; AgentMemory is transport.
- Save a non-secret AgentMemory handoff after material changes.
- Never commit content-signing private keys, relay tokens, session URLs, VM credentials, imported payload bundles, or release working directories.

## Non-negotiable product rules

- Product version for this release is exactly `0.0.1zoro`.
- Do not edit upstream exploit/payload bytes inside ron1n. Import them as pinned external bundles.
- Keep firmware profiles isolated. A nearby firmware is not compatible merely because paths look similar.
- Local Application Cache and relative module routes must remain byte/path compatible.
- Remote delivery is an allowlisted static-content relay, never a shell, scanner, arbitrary proxy, or upload-and-execute channel.
- Report request, partial transfer, full transfer, and console execution as distinct states.
- Require HTTPS for production remote mode and signatures for active bundles.
- Runtime code remains Go; Bash and PowerShell are bootstrap/automation only.
- Keep Linux, Windows, and macOS amd64/arm64 cross-builds green.

## Required verification

Run at least:

```bash
gofmt -w cmd internal
go test ./...
go test -race ./...
go vet ./...
bash -n ron1n install.sh scripts/build-release.sh
make release VERSION=0.0.1zoro
```

Verify PowerShell parsing and the documented `irm | iex` path in the Windows 10 QEMU/KVM guest. Verify the pinned real PSFree bundle's cache MIME, full `goldhen.bin` SHA-256, and range response. Hardware kernel-execution claims require the actual PS4 9.00 console.
