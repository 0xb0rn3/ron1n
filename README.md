# ron1n v1.3.0

**PS4 9.00 Local Jailbreak Host Orchestrator for Arch Linux**  
Built by **0xb0rn3 / Stingray Labs**.

## v1.3 — browser-aware PS4 detection

The old detector depended mostly on:

- ping
- ARP vendor/OUI matching
- a remembered MAC address

That can miss a PS4 even while the console is actively using the jailbreak host.

v1.3 fixes that by replacing `python -m http.server` with the included **`ron1n-server.py`**.

Every request now records:

- source IP
- HTTP User-Agent
- requested path
- timestamp
- whether the client identifies as a PlayStation
- observed exploit-host stage

When the PS4 browser requests ron1n, its PlayStation browser User-Agent becomes the primary detection signal.

## Detection priority

```text
1. PS4 browser HTTP/User-Agent fingerprint
2. Previously learned PS4 MAC
3. Sony/PlayStation ARP vendor match
```

The HTTP method is much stronger because the console is literally connecting to ron1n.

## Activity states

ron1n recognizes requests associated with:

```text
offline-cache
PS4 browser
PSFree
Lapse
goldhen.bin
```

For example:

```text
[✓] PS4 detected at 192.168.1.55
[+] PS4: PSFree activity
[+] PS4: Lapse activity
[✓] PS4: GoldHEN payload served
```

**Important:** `GoldHEN payload served` means ron1n successfully delivered the payload file to the PS4. A web server cannot prove that the payload subsequently executed successfully inside the console kernel. Check the console UI for final GoldHEN success.

## Install

Keep both files together:

```text
ron1n
ron1n-server.py
```

Then:

```bash
chmod +x ron1n ron1n-server.py
./ron1n
```

The installer copies the server component into:

```text
~/.local/share/ron1n/ron1n-server.py
```

and systemd runs that copy.

## Live activity

Use:

```bash
./ron1n watch
```

This gives you a live terminal view of the last PlayStation HTTP event.

## Status

```bash
./ron1n status
```

Example:

```text
[✓] ron1n.service is running.

[✓] PS4 browser detected: 192.168.1.55
    State : GoldHEN payload served
    Path  : /goldhen.bin
    Seen  : 2s ago
```

## State files

```text
~/.local/state/ron1n/console.json
~/.local/state/ron1n/events.log
~/.local/state/ron1n/ps4.ip
~/.local/state/ron1n/ps4.mac
```

`events.log` retains request events while `console.json` contains the most recent PlayStation event.

## PS4 connection

Both devices need to be on the same LAN.

On the console:

```text
Settings
→ Network
→ Set Up Internet Connection
→ Use Wi-Fi / Use a LAN Cable
→ same router as ron1n host
→ Easy
```

Then open ron1n in the PS4 browser.

## Bookmark setup

First visit:

```text
http://ARCH-IP:8080/cache.html
```

After caching, bookmark:

```text
http://ARCH-IP:8080/
```

as:

```text
★ Jailbreak
```

## Commands

```bash
./ron1n
./ron1n wait
./ron1n watch
./ron1n status
./ron1n connect
./ron1n restart
./ron1n logs
./ron1n update
```

## Attribution

ron1n is the **Stingray Labs orchestration/hosting layer**.

PSFree, Lapse, GoldHEN and their associated components remain the work of their respective upstream authors and retain their respective licenses and notices.

---

**ron1n**  
Built by **0xb0rn3 / Stingray Labs**
