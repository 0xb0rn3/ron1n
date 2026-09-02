# Upstream and exploit-chain audit

Audited: 2026-09-02

## Active upstream contract

Legacy ron1n downloads `https://github.com/kmeps4/PSFree`. Its audited `main` commit is `368d82aa40d3017c220757ce315761adb5f06678`. Release `0.0.1zoro` pins that full commit by default instead of pulling a mutable branch.

That tree contains PSFree, root-level Lapse, the 9.00 ROP and kernel patch material, `aio_patches.bin`, an Application Cache manifest, and `goldhen.bin`. The request chain and exact served bytes are treated as an external compatibility fixture. ron1n does not rewrite its exploitation logic.

Observed required sequence:

1. `index.html` reads `window.applicationCache.status` and redirects an uncached browser to `cache.html`.
2. `cache.html` declares `psfree_lapse.cache` and reports Application Cache progress.
3. The cache manifest lists HTML, modules, font, ROP, kernel patch, AIO patch, and GoldHEN resources using relative paths.
4. `alert.mjs` imports `psfree.mjs`; PSFree imports `lapse.mjs`.
5. Lapse fetches patch material and, after its kernel path, starts binary fetches for `aio_patches.bin` and `goldhen.bin`.

The upstream calls `PayloadLoader(...)` while constructing `setTimeout`, so the two binary requests may overlap immediately. Both local hosting and the remote relay must therefore allow at least four concurrent requests and preserve path prefixes, bodies, MIME, conditional and range behavior.

## Integrity observations

- `kmeps4/PSFree` declares AGPL-3.0-or-later; its Liberation font carries OFL-1.1 notices.
- The audited `goldhen.bin` is 290,016 bytes with observed SHA-256 `c6329401d1810e16c84e6474ac30977dbdc951987c10cdb559370de7d59db0b0`.
- The upstream commit message calls that binary 2.4b18.10, but official GoldHEN GitHub metadata does not independently corroborate that version.
- `GoldHEN/GoldHEN` exposes releases but has no repository license file and says its source is private. Public availability is not assumed to grant redistribution or modification rights.
- The latest audited official release is prerelease `2.4b18`; its archive had observed SHA-256 `d0c84c79f65df5afc79a00c578f33ab1aa70aeb9c205f1e789895dc7d4fca38d`, but GitHub provided no publisher checksum or signature. This is an observation, not publisher authentication.
- GitHub content API blob SHA-1 values are not treated as payload SHA-256 values.

Consequently, ron1n keeps payload delivery private/session-scoped, preserves notices and corresponding source, records provenance, pins full revisions, and adds its own Ed25519 chain-of-custody manifest. This signature states what the operator imported; it does not impersonate an upstream signature.

## Known upstream issue

The audited `about.html` links `./scripts/lapse.mjs`, while the file is at `/lapse.mjs`. ron1n does not alter the exploit tree during import. The issue is documented so a future upstream-derived profile can correct the corresponding-source link with the required AGPL modification notice and hardware regression testing.

## Alternate lineages

`Al-Azif/psfree-lapse` is a separately maintained combined implementation, audited at commit `08ecf038c94aa99b56e46c9f32e2e486f83656b6` and tagged `v1.5.1`. It is not silently swapped for the kmeps4 tree. Firmware-specific exploit chains are content profiles with independent manifests and hardware acceptance, not interchangeable server themes.

## Telemetry semantics

An HTTP request proves only browser activity. A completed, hash-matching response proves only that ron1n transferred bytes to the client connection. Neither proves that the exploit, kernel patch, AIO patch, or GoldHEN executed successfully. The CLI and event schema keep those states separate.
