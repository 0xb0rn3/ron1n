# ron1n `0.0.1zoro` architecture

## Runtime topology

Local mode:

```text
PS4 browser --HTTP/LAN--> ron1n host --read-only--> verified content bundle
                                      `--append--> bounded local event state
```

Remote mode:

```text
PS4 browser --HTTPS--> ron1n-relay <--outbound HTTPS poll/respond-- ron1n agent
                              |                                      |
                    expiring session                         verified bundle
```

The outbound agent model works behind NAT and does not require the console and host to share a LAN. The first protocol uses bounded HTTPS long polling so both binaries remain Go-standard-library-only. The protocol can later gain a multiplexed stream without changing the content or session model.

## Components

- `cmd/ron1n`: operator CLI, local HTTP host, outbound relay agent, bundle importer, integrity tools, status/watch commands, and platform integration.
- `cmd/ron1n-relay`: self-hostable browser relay, session broker, host credential provisioner, and health endpoint.
- `internal/content`: deterministic manifests, Ed25519 signatures, safe archive import, exact route metadata, hashing, and allowlisted file resolution.
- `internal/host`: PS4-compatible HTTP semantics and completion-aware event recording.
- `internal/relay`: authenticated host API, expiring browser sessions, request broker, and agent client.
- `internal/state`: locked/atomic compatibility state and bounded JSONL events.
- `internal/platform`: XDG/Windows known paths and opt-in background-task integration.

## PSFree/Lapse/GoldHEN compatibility

The audited upstream revision is recorded in `docs/UPSTREAM_AUDIT.md`. Its execution path is:

1. `/` checks the legacy browser Application Cache state and redirects an uncached console to `cache.html`.
2. `cache.html` loads `psfree_lapse.cache`; every listed path is relative to that manifest URL.
3. `alert.mjs` imports `psfree.mjs`.
4. PSFree imports `lapse.mjs` after WebKit exploitation.
5. Lapse fetches kernel patch material.
6. After the kernel path succeeds, the page initiates requests for `aio_patches.bin` and `goldhen.bin`.

ron1n does not modify those steps. It verifies and transports their bytes. Local and remote request prefixes must remain stable so relative imports and Application Cache entries resolve identically.

## Relay protocol v1

Host authentication uses a random bearer credential whose SHA-256 digest is stored by the relay. Browser access uses a separate random capability token; only its digest is kept in relay memory.

Host API:

- `GET /v1/agent/poll?host_id=...`: authenticated long poll for one `GET`/`HEAD` request.
- `POST /v1/agent/respond?host_id=...&request_id=...`: authenticated bounded response. The call returns only after the browser write completes or fails.
- `POST /v1/agent/sessions?host_id=...`: create an expiring browser session.
- `DELETE /v1/agent/sessions/{id}?host_id=...`: revoke a session.

`ron1n relay session` prints the non-capability session ID alongside the secret browser URLs. `ron1n relay revoke --session ID` authenticates with the configured host credential and calls the delete endpoint, allowing an exposed or finished session to be invalidated without restarting the relay.

Browser API:

- `GET|HEAD /s/{capability}/...`: forward an allowlisted request to its host.

Only `Range`, `If-None-Match`, and `If-Modified-Since` cross the tunnel. Cookies, authorization headers, arbitrary URLs, POST bodies, and hop-by-hop headers do not.

## Trust domains

Transport credentials authenticate a host to a relay. Separately generated bundle-signing keys authorize imported content. Compromising either does not silently grant the other capability.

Application releases have a narrower trust model in `0.0.1zoro`: the Bash and PowerShell bootstrap scripts verify each downloaded binary against `SHA256SUMS`, but the scripts, checksums, and binaries are all obtained from the same tagged GitHub repository/release trust domain. The content Ed25519 key does not sign application releases. There is no application self-updater, independently rooted release-signing key, or application rollback engine in this version. Those are future work; `ron1n update` updates imported content only.

## Deployment

For production, bind `ron1n-relay` to loopback behind a public-CA HTTPS reverse proxy, or provide a certificate and key directly. The configured external URL—not an untrusted Host header—is used to create browser links. Cloudflare Tunnel, Caddy, nginx, or another HTTPS ingress may be used, but ron1n has no provider lock-in.
