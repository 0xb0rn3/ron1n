# `0.0.1zoro` release runbook

This runbook records and reproduces the exact tagged ron1n releases consumed by the Bash and PowerShell bootstrap installers. GitHub push and GitHub release publication are separate operations: pushing `main` alone does not make installer asset URLs work.

Current publication state:

- `0.0.1zoro`: original product release; Linux/macOS installer default.
- `0.0.1zoro-r1`: immutable Windows Scheduled Task quoting correction.
- `0.0.1zoro-r2`: current Windows distribution revision with stop-before-delete uninstall; built from `acd7acb3f62ce099d1c792b993b8145de8240f0a`.

Every release above has 13 uploaded assets. All binaries report product version `0.0.1zoro`; `r1` and `r2` are distribution revisions, not different product-version strings.

## Trust boundary

- Imported PSFree/Lapse content is inventoried byte by byte and signed with the operator's local Ed25519 content key.
- ron1n application binaries are checked against `SHA256SUMS` downloaded from the same tagged GitHub release.
- The checksum catches corruption or a mismatched asset. It is not an independent application-release signature because the installer, checksum, and binaries share the GitHub trust domain.
- `0.0.1zoro` has no application self-updater or automatic application rollback. Independently signed releases and rollback-aware self-update are future work.

## Required uploaded assets

The release has 13 uploaded files: 12 native binaries and one checksum file.

```text
ron1n-linux-amd64
ron1n-linux-arm64
ron1n-windows-amd64.exe
ron1n-windows-arm64.exe
ron1n-darwin-amd64
ron1n-darwin-arm64
ron1n-relay-linux-amd64
ron1n-relay-linux-arm64
ron1n-relay-windows-amd64.exe
ron1n-relay-windows-arm64.exe
ron1n-relay-darwin-amd64
ron1n-relay-darwin-arm64
SHA256SUMS
```

GitHub's automatic source archives are not part of this count.

## Build and publish

Run these steps from the complete release commit as 0xb0rn3. Do not build release artifacts from a dirty tree.

```bash
release_version=0.0.1zoro

git status --short
go test ./...
go test -race ./...
go vet ./...
bash -n ron1n install.sh scripts/build-release.sh

test -z "$(git status --porcelain)"
git push origin main
git tag -a "$release_version" -m "ron1n $release_version"
git push origin "$release_version"

make release VERSION="$release_version"
(cd "dist/$release_version" && sha256sum -c SHA256SUMS)
test "$(find "dist/$release_version" -maxdepth 1 -type f | wc -l)" -eq 13

gh release create "$release_version" \
  "dist/$release_version/"* \
  --repo 0xb0rn3/ron1n \
  --verify-tag \
  --title "ron1n $release_version" \
  --notes-file CHANGELOG.md
```

If `sha256sum` is unavailable on the release host, verify with `shasum -a 256 -c SHA256SUMS` from inside the release directory.

Never move a published tag or replace one of its assets. Correct a released artifact by publishing a new distribution revision or product version with a new tag and checksum set.

## Post-publication checks

Confirm that GitHub exposes exactly 13 uploaded assets:

```bash
gh release view 0.0.1zoro-r2 \
  --repo 0xb0rn3/ron1n \
  --json tagName,isDraft,isPrerelease,assets \
  --jq '{tag: .tagName, draft: .isDraft, prerelease: .isPrerelease, assets: (.assets | length)}'
```

Confirm the tag-pinned bootstrap scripts and representative release files are reachable:

```bash
for url in \
  'https://raw.githubusercontent.com/0xb0rn3/ron1n/0.0.1zoro/install.sh' \
  'https://raw.githubusercontent.com/0xb0rn3/ron1n/acd7acb3f62ce099d1c792b993b8145de8240f0a/install.ps1' \
  'https://github.com/0xb0rn3/ron1n/releases/download/0.0.1zoro-r2/SHA256SUMS' \
  'https://github.com/0xb0rn3/ron1n/releases/download/0.0.1zoro-r2/ron1n-windows-amd64.exe' \
  'https://github.com/0xb0rn3/ron1n/releases/download/0.0.1zoro/SHA256SUMS' \
  'https://github.com/0xb0rn3/ron1n/releases/download/0.0.1zoro/ron1n-linux-amd64' \
  'https://github.com/0xb0rn3/ron1n/releases/download/0.0.1zoro/ron1n-relay-windows-amd64.exe'; do
  curl --fail --silent --show-error --location --output /dev/null "$url"
done
```

Perform an isolated Linux installer smoke test:

```bash
release_smoke_dir="$(mktemp -d)"
curl -fsSL 'https://raw.githubusercontent.com/0xb0rn3/ron1n/0.0.1zoro/install.sh' |
  RON1N_INSTALL_DIR="$release_smoke_dir" bash
"$release_smoke_dir/ron1n" version
"$release_smoke_dir/ron1n-relay" version
```

Both commands must print exactly `0.0.1zoro`. Remove the isolated directory after reviewing its contents.

Finally, run the immutable commit-pinned installer and smoke suite in the Windows 10 QEMU/KVM guest:

```powershell
irm 'https://raw.githubusercontent.com/0xb0rn3/ron1n/acd7acb3f62ce099d1c792b993b8145de8240f0a/install.ps1' | iex
irm 'https://raw.githubusercontent.com/0xb0rn3/ron1n/d9be416c4c6d44e054ae60ac0f29ba688a412e17/scripts/windows-vm-smoke.ps1' | iex
```

Record the checksum verification, version output, PATH behavior, content import, local host, relay delivery, explicit `ron1n relay revoke --session ID` result, and execution-policy result in `TESTING.md` and `BUILD_STATUS.md`.

The first tag-pinned Windows bootstrap exposed a real compatibility defect during this gate: PowerShell 5.1 on the Windows 10 guest did not expose `RuntimeInformation.OSArchitecture`. The tag and release assets were left immutable. Commit `d4a8d5913768735ea75683876e78c4e62900d6ad` added the legacy-safe `PROCESSOR_ARCHITECTURE`/`PROCESSOR_ARCHITEW6432` fallback; the current command above pins the later reviewed `r2` commit containing that fix.

The first full smoke run then exposed a harness-only null-log diagnostics bug after all local checks and outbound-agent startup had passed. Commit `d9be416c4c6d44e054ae60ac0f29ba688a412e17` makes empty redirected logs safe and is the immutable smoke-script reference above.

The Windows service checks exposed two integration defects in sequence. Revision `0.0.1zoro-r1` fixes action quoting so `schtasks.exe` stores a valid executable and argument pair. Revision `0.0.1zoro-r2` ends a running task before deleting it so uninstall cannot leave the host process listening after autostart removal. The final Windows 10 QEMU/KVM acceptance passed install, health, restart, content update, uninstall, repeat uninstall, and execution-policy preservation against `r2`.
