# PS4 host and delivery ecosystem audit

Audited: 2026-09-02

The goal was to cover the discoverable backend/delivery families related to ron1n, not to merge every public exploit fork. GitHub repository searches used combinations of `PS4 exploit host`, `PSFree Lapse GoldHEN`, and `PS4 Server 900u`; results were then narrowed to repositories with a distinct serving, caching, update, relay, appliance, DNS, or embedded delivery mechanism. `ron1n sources` prints the maintained catalog in the binary.

## Mechanisms incorporated

- From kmeps4/PSFree: the exact 9.00 Application Cache, module, Lapse, AIO patch, and GoldHEN route contract.
- From Al-Azif/ps4-exploit-host: separation of host configuration, static export, cache-aware responses, public-mode restrictions, and explicit delivery timeouts. ron1n does not copy unrestricted request forwarding or automatic payload dispatch.
- From Al-Azif/exploit-host-dns: DNS is a separable optional deployment component, not a prerequisite for HTTP delivery.
- From ArabPixel/WebKitty: firmware-specific cache manifests and content profiles. A profile cannot silently reuse another firmware's patch or exploit chain.
- From Home Assistant PS4 JB hosts: persistent content and convenient updates, changed to staged/hash/signature-verified activation instead of an unauthenticated public admin upload.
- From ESP32/Buildroot host families: constrained, offline-capable serving and clear separation between transport completion and console execution.
- From remote static hosts: the console may consume content without sharing a LAN with the operator machine. ron1n adds expiring capability sessions and an outbound agent rather than making the home host publicly reachable.

## Deliberately excluded mechanisms

- LAN scanning, OUI guessing, and spoofable MAC identity are not authorization.
- Automatic payload sends to an observed IP are not part of the relay.
- The relay has no shell, arbitrary process, CONNECT, arbitrary URL proxy, filesystem browse, or upload-and-execute operation.
- DNS hijacking and update-domain blocking are not enabled by default and are not necessary for remote delivery.
- Repositories without a license are reference-only unless the operator separately establishes permission.
- Exploit/payload trees are never blended merely because they target a nearby firmware number.

## Catalog scope

The catalog covers these distinct families:

- Canonical/current content: kmeps4/PSFree.
- Alternate PSFree/Lapse lineage: Al-Azif/psfree-lapse.
- Official payload and loader references: GoldHEN/GoldHEN and GoldHEN/henloader_lp.
- General-purpose host/control plane: Al-Azif/ps4-exploit-host and exploit-host-dns.
- Modern multi-firmware static host: ArabPixel/WebKitty and its PET test tree.
- On-console HTTP backend: ArabPixel/ps4-websrv.
- Home automation appliance: muratcesmecioglu/ha-ps4-jb.
- Linux/USB appliance: Shivelight/pOOBs4-buildroot and stooged/PS4-Server-900u.
- Embedded host/USB delivery: stooged/ESP32-Server-900u, ionontelodico/GoldHEN-Loader-ESP32, and frwololo/PS4_PS5-ESP8266-Server.

Many search results are mirrors, branded static pages, guides, or unlicensed payload collections implementing no new backend mechanism. They are not copied into ron1n. The generic signed static-bundle importer is the extension point for a user-authorized repository; a new firmware profile still requires license review, a pinned revision, exact route metadata, automated cache tests, and real hardware acceptance.
