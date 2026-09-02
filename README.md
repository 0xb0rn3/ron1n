# ron1n `0.0.1zoro`

ron1n is a native, cross-platform PS4 firmware 9.00 jailbreak-content host built by 0xb0rn3 / Stingray Labs. It imports a pinned PSFree/Lapse host, verifies every served byte, preserves the browser's offline-cache and exploit request sequence, records honest transfer telemetry, and can deliver the same content when the console and operator host are on different networks.

The ron1n orchestration, HTTP server, state engine, importer, relay, CLI, and service integration are Go. Python is gone. Bash and PowerShell are supported bootstrap and automation layers, not the runtime. The release provides native binaries for Linux, Windows, and macOS on amd64 and arm64.

ron1n does not rewrite or claim authorship of PSFree, Lapse, the ROP/kernel patches, AIO patches, GoldHEN, or other upstream payloads. Those remain separately attributed external components.

## Install with one command

The copy-paste installers download both `ron1n` and `ron1n-relay` from the `0.0.1zoro` GitHub release, select the correct OS/architecture, fetch `SHA256SUMS`, and refuse installation if either binary does not match.

### Windows — current user

Open PowerShell and run:

```powershell
irm 'https://raw.githubusercontent.com/0xb0rn3/ron1n/0.0.1zoro/install.ps1' | iex
```

This installs to `%LOCALAPPDATA%\ron1n\bin` and adds that directory to the current user's PATH. It does not require administrator rights and does not change the machine's PowerShell execution policy.

### Windows — elevated machine-wide install

Run this from a normal PowerShell window. Windows will show its standard UAC prompt:

```powershell
Start-Process powershell.exe -Verb RunAs -ArgumentList '-NoProfile -ExecutionPolicy Bypass -Command "$env:RON1N_INSTALL_SCOPE=''Machine''; irm ''https://raw.githubusercontent.com/0xb0rn3/ron1n/0.0.1zoro/install.ps1'' | iex"'
```

This uses `-ExecutionPolicy Bypass` only for the new installer process; it does not weaken or overwrite the permanent user or machine policy. Machine scope installs to `%ProgramFiles%\ron1n` and updates the machine PATH.

### Linux — current user

```bash
curl -fsSL 'https://raw.githubusercontent.com/0xb0rn3/ron1n/0.0.1zoro/install.sh' | bash
```

This installs to `~/.local/bin` without root.

### Linux — system-wide

```bash
curl -fsSL 'https://raw.githubusercontent.com/0xb0rn3/ron1n/0.0.1zoro/install.sh' | sudo bash -s -- --system
```

This installs to `/usr/local/bin`.

### macOS — current user

```bash
curl -fsSL 'https://raw.githubusercontent.com/0xb0rn3/ron1n/0.0.1zoro/install.sh' | bash
```

The installer detects Intel versus Apple Silicon and installs to `~/.local/bin`.

### Inspect before executing

`irm | iex` and `curl | bash` intentionally execute code fetched from GitHub. If you prefer to inspect it first:

Windows:

```powershell
$installer = Join-Path $env:TEMP 'ron1n-install.ps1'
irm 'https://raw.githubusercontent.com/0xb0rn3/ron1n/0.0.1zoro/install.ps1' -OutFile $installer
Get-Content $installer
& $installer
```

Linux/macOS:

```bash
curl -fsSLo /tmp/ron1n-install.sh 'https://raw.githubusercontent.com/0xb0rn3/ron1n/0.0.1zoro/install.sh'
less /tmp/ron1n-install.sh
bash /tmp/ron1n-install.sh
```

The installer URL is pinned to the `0.0.1zoro` tag. The scripts, release binaries, and `SHA256SUMS` still share the GitHub repository trust domain: the checksum detects transfer or storage corruption, but it is not an independently signed release attestation. Inspect the tag/commit and compare the published checksums through an independent trusted channel when stronger authentication is required.

The local Ed25519 key used for imported content does not sign ron1n application binaries. Release `0.0.1zoro` has no application self-updater, release-signing key, or automatic application rollback. Upgrade the application by reviewing and running the installer for a newer explicit release. Independently signed application releases and rollback-aware self-update remain future work.

## Quick start: same-LAN/offline mode

Install and initialize the pinned content bundle:

```bash
ron1n install
ron1n serve
```

Or initialize and enable native user autostart in one explicit command:

```bash
ron1n install --service
```

`--service` uses a systemd user unit on Linux, a native Scheduled Task on Windows, and a LaunchAgent on macOS. ron1n does not silently install packages, enable lingering, modify the firewall, or disable an older service.

The CLI prints the detected LAN URL. On the PS4:

1. Confirm the console is on firmware 9.00.
2. Open the browser at `http://HOST-IP:8080/cache.html`.
3. Wait for the upstream offline-cache page to report completion.
4. Open `http://HOST-IP:8080/` and bookmark it.

The default port remains `8080`. Override it with `RON1N_PORT` or the JSON configuration.

## Different-network delivery — no shared LAN required

Remote mode keeps the operator host behind NAT. It creates outbound HTTPS requests to a self-hosted relay; no inbound home port, public home IP, LAN scan, or shared router is required.

```text
PS4 browser --HTTPS--> ron1n-relay <--outbound authenticated HTTPS-- ron1n agent
                              |                                      |
                    expiring capability                       verified bundle
```

### 1. Provision a host on the relay machine

```bash
ron1n-relay provision \
  --host my-ps4-host \
  --credentials /etc/ron1n/relay-hosts.json \
  --token-out ./my-ps4-host.token
```

The relay stores only the SHA-256 digest of the host token. Transfer the token file privately to the ron1n host, then protect it as a secret.

### 2. Run the relay behind HTTPS

Recommended reverse-proxy deployment:

```bash
ron1n-relay serve \
  --listen 127.0.0.1:9090 \
  --external-url 'https://relay.example.com' \
  --credentials /etc/ron1n/relay-hosts.json
```

Point a public-CA HTTPS reverse proxy or tunnel at `127.0.0.1:9090`. Caddy, nginx, Cloudflare Tunnel, and comparable ingress can work; ron1n does not depend on one provider. Configure the proxy not to log `/s/<capability>/...` tokens and not to CDN-cache session paths.

Alternatively, terminate TLS inside `ron1n-relay` with `--tls-cert` and `--tls-key`. A plaintext public bind is refused unless the explicit development-only override is supplied.

### 3. Configure and run the outbound host agent

```bash
ron1n relay configure \
  --url 'https://relay.example.com' \
  --host-id my-ps4-host \
  --token-file '/secure/path/my-ps4-host.token'

ron1n relay connect
```

The default four outbound workers preserve the audited upstream's overlapping module, AIO patch, and GoldHEN fetches.

### 4. Create the console browser URL

In another terminal on the host:

```bash
ron1n relay session --ttl 30m
```

The command prints a session ID plus the root and `/cache.html` HTTPS URLs. Open the cache URL on the PS4. The capability is high-entropy, expires, has request/byte limits, and exists only in relay memory. Treat the URL as a secret and retain the session ID if you may need to revoke it.

### 5. Revoke a console browser URL

Revoke a session immediately when it is no longer needed or its capability URL may have been exposed:

```bash
ron1n relay revoke --session SESSION_ID
```

This uses the configured host credential to invalidate that session. The session ID is printed by `ron1n relay session`; it is not the browser capability token.

The relay forwards only `GET` and `HEAD` requests for files in the active verified manifest. It is not a VPN or general-purpose reverse proxy and has no remote shell, command, process, arbitrary URL, directory browse, LAN scan, arbitrary upload, or upload-and-execute message.

## What the jailbreak delivery path does

The default content profile pins `kmeps4/PSFree` at commit `368d82aa40d3017c220757ce315761adb5f06678`.

The audited browser sequence is:

1. `index.html` checks the PS4 browser's legacy Application Cache state.
2. An uncached browser visits `cache.html`, which loads `psfree_lapse.cache`.
3. The manifest caches the HTML, ES modules, font, ROP data, kernel patch material, `aio_patches.bin`, and `goldhen.bin` using relative URLs.
4. `alert.mjs` loads PSFree, the browser exploitation stage.
5. PSFree loads Lapse, the kernel exploitation stage.
6. After that path succeeds, upstream requests the AIO patch and GoldHEN binaries. Those requests may overlap.

ron1n preserves the URL prefix, exact bytes, `text/cache-manifest` and JavaScript MIME types, concurrent requests, ETags, conditional GETs, `HEAD`, and single byte ranges in local and relay modes. It does not edit the imported exploit code.

Before activation, the importer:

- resolves the requested GitHub ref to a full 40-character commit;
- limits archive and extracted sizes;
- rejects traversal, absolute paths, symlinks, hard links, devices, duplicate writes, and `.git` metadata;
- validates required firmware-9.00 files, including the previously missed `aio_patches.bin`;
- records every file's path, MIME, size, stage, and SHA-256 in a deterministic manifest;
- signs that manifest with the operator's local Ed25519 content key;
- verifies every byte again before serving; and
- activates a new immutable bundle without overwriting the previous one.

## Honest status and activity semantics

Legacy ron1n labeled `goldhen.bin` as served before the HTTP response and could produce the same label for `HEAD`, a 404 path, or an interrupted transfer. It also defined a 30-second freshness limit but never enforced it.

`0.0.1zoro` records this progression:

```text
artifact-requested
  -> head-complete
  -> artifact-partial
  -> artifact-delivered
  -> response-failed
```

Only an allowlisted, hash-matching, successful full-body `GET` can become `artifact-delivered`. In remote mode, the agent waits for the relay to acknowledge the browser write before recording completion.

Even `artifact-delivered` means only that ron1n transferred the bytes. It does not prove PSFree succeeded, Lapse gained kernel access, the AIO patch ran, or GoldHEN executed. Real execution validation requires the console UI and a firmware-9.00 hardware test.

PlayStation User-Agent and IP data are activity observations, never credentials. The stale-state limit is now enforced. Automatic ARP/OUI scanning was removed.

## Commands

```text
ron1n install [--service]              initialize and import pinned content
ron1n serve                            run the verified local host
ron1n status [--json]                  service, health, bundle, and recent state
ron1n wait                             stop after new PlayStation activity
ron1n watch [--json]                   continuously show activity
ron1n connect                          print local and remote instructions
ron1n restart                          restart native autostart integration
ron1n logs [--follow=false]            read the bounded JSONL event log
ron1n update                           update imported content from upstream HEAD; not the app binary
ron1n content sync                     import a pinned GitHub content tree
ron1n content build                    create a deterministic content manifest
ron1n content verify                   verify signature and all content bytes
ron1n keys generate                    create an Ed25519 content-signing pair
ron1n relay configure                  save non-secret relay settings
ron1n relay connect                    run outbound cross-network workers
ron1n relay session --ttl 30m          create an expiring browser capability
ron1n relay revoke --session ID        revoke a browser capability by session ID
ron1n sources [--json]                 list audited related delivery repositories
ron1n doctor                           run local integrity and health checks
ron1n uninstall                        remove autostart; preserve user data and keys
ron1n version                          print 0.0.1zoro
```

Legacy environment compatibility:

- `RON1N_REPO_URL`
- `RON1N_PORT`
- `RON1N_RECENT_HTTP_SECONDS`
- `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, and `XDG_CACHE_HOME` on Linux

Additional variables include `RON1N_CONTENT_DIR` and `RON1N_RELAY_URL`.

## Data locations

Linux follows XDG defaults:

- config and keys: `~/.config/ron1n`
- bundles: `~/.local/share/ron1n`
- state/events: `~/.local/state/ron1n`
- cache: `~/.cache/ron1n`

Windows:

- config: `%APPDATA%\ron1n`
- data/state/cache: `%LOCALAPPDATA%\ron1n`

macOS:

- config/data/state: `~/Library/Application Support/ron1n`
- cache: `~/Library/Caches/ron1n`

Signing private keys and relay bearer tokens are stored separately from `config.json`. Session tokens are redacted by ron1n logs. Event logs rotate instead of growing forever. `/_ron1n/health` reveals only health, version, and bundle ID; the old public client-detail status endpoint is gone.

## Related hosts and delivery backends

ron1n's ecosystem audit covers the distinct discoverable mechanism families rather than bulk-copying branded pages or unlicensed payload mirrors:

- Al-Azif exploit host and separate DNS backend
- Al-Azif's combined PSFree/Lapse lineage
- GoldHEN and its maintained loader references
- ArabPixel WebKitty, PET, and on-console `ps4-websrv`
- Home Assistant PS4 JB hosting
- stooged PC/ESP32 9.00 USB-host families
- Buildroot appliance hosting
- ESP32/ESP8266 direct and embedded delivery families

Run `ron1n sources` for the built-in catalog, license status, role, and inclusion policy. Read [the upstream audit](docs/UPSTREAM_AUDIT.md) and [ecosystem audit](docs/ECOSYSTEM_AUDIT.md) for the mechanism comparison and exact scope.

Useful ideas incorporated include firmware-specific cache profiles, isolated DNS, static export compatibility, persistent staged content, constrained-device behavior, and remote static hosting. Deliberately excluded ideas include automatic LAN scanning, spoofable identity, unrestricted forwarding, public admin uploads, silent firmware-chain substitution, and public payload mirroring.

## Build and test from source

Go 1.24 or newer is required to build. The runtime itself has no third-party Go modules and uses `CGO_ENABLED=0` release binaries.

```bash
git clone 'https://github.com/0xb0rn3/ron1n.git'
cd ron1n
make test
make race
make vet
make build
```

Build all 12 release assets plus `SHA256SUMS`:

```bash
make release VERSION=0.0.1zoro
```

Release publication is a separate, manual gate. Commit and push the complete tree, create the exact `0.0.1zoro` tag on that clean commit, build from the clean tagged tree, verify `SHA256SUMS`, and attach the 12 binaries plus `SHA256SUMS` to the matching GitHub release. Do not move the tag or replace published assets; issue a new version instead. The exact commands and post-publication URL checks are in [the release runbook](docs/RELEASE.md).

The matrix is:

- Linux amd64/arm64
- Windows amd64/arm64
- macOS amd64/arm64
- both `ron1n` and `ron1n-relay`

The source-tree `./ron1n` file is now only a Bash compatibility launcher. It executes a local built binary or `go run ./cmd/ron1n`; no hosting logic remains in Bash.

## Security and legal boundaries

- Use ron1n only with hardware and content you are authorized to operate.
- Remote mode is disabled until configured and requires HTTPS outside loopback development.
- A relay operator or TLS terminator remains trusted for browser delivery; content signatures establish host chain-of-custody but cannot make an old browser verify Ed25519 itself.
- The default relay keeps sessions in memory; `ron1n relay revoke --session ID` invalidates one explicitly, and a relay restart revokes all of them.
- ron1n never claims that a transferred payload executed.
- `kmeps4/PSFree` declares AGPL-3.0-or-later and includes an OFL-1.1 font. ron1n preserves the imported upstream tree and notices without editing its bytes. The audited upstream `about.html` contains a broken Lapse source link; the actual root-level `lapse.mjs` is still preserved, and the defect is recorded in the upstream audit.
- `GoldHEN/GoldHEN` had no repository license in the 2026-09-02 audit. Public availability is not treated as permission to modify or operate a public mirror. Prefer user-supplied or explicitly fetched official material and private/session-scoped delivery.
- This ron1n repository does not currently declare a license for its own new Go code. No permission beyond applicable law is implied until 0xb0rn3 selects one.

## Documentation and coordination

- [Architecture](ARCHITECTURE.md)
- [Decisions and ownership](DECISION.md)
- [Build status](BUILD_STATUS.md)
- [Release runbook](docs/RELEASE.md)
- [Upstream audit](docs/UPSTREAM_AUDIT.md)
- [Ecosystem audit](docs/ECOSYSTEM_AUDIT.md)

Built by **0xb0rn3 / Stingray Labs**.
