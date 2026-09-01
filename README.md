# ron1n v1.2.0

**PS4 9.00 Local Jailbreak Host Orchestrator for Arch Linux**

Built by **0xb0rn3 / Stingray Labs**.

## What changed in 1.2.0

This release fixes the startup failure encountered after migrating from the earlier PS4 host prototype.

The old prototype created:

```text
ps4-900-host.service
```

and listened on:

```text
TCP/8080
```

`ron1n` also uses TCP/8080. If the old service was still running, the new `ron1n.service` could not bind to that port.

v1.2.0 now:

- automatically detects and disables known legacy services
- checks TCP/8080 before starting
- refuses to kill unrelated processes
- prints the actual systemd journal if startup fails
- suppresses normal `pacman` installation noise
- suppresses normal Git update noise
- includes the PS4-side connection instructions in the UI
- waits indefinitely for the PS4 if it is offline
- remembers the PS4 IP and MAC

## Install

```bash
chmod +x ron1n
./ron1n
```

Normal package installation now appears simply as:

```text
[+] Setting up ron1n...
[✓] System dependencies ready.
```

Full pacman output is only shown when dependency setup fails.

## Connecting the PS4

When the console is not detected, ron1n displays:

```text
Connect the PS4 to ron1n

PS4 → Settings → Network → Set Up Internet Connection
• Use Wi-Fi or Use a LAN Cable
• Connect to the same router/LAN as the Arch machine
• Choose Easy
• Leave the console powered on
```

The console and Arch host do not need a direct cable between them. They only need to be on the same LAN and the router must permit communication between clients.

## Browser setup

Once the PS4 is detected, ron1n prints:

```text
http://ARCH-IP:8080/cache.html
http://ARCH-IP:8080/
```

On the PS4:

1. Open Internet Browser.
2. Visit `/cache.html`.
3. Allow the offline cache to complete.
4. Open the main `/` page.
5. Press **OPTIONS → Add Bookmark**.
6. Name the bookmark **★ Jailbreak**.

## Commands

```bash
./ron1n
./ron1n wait
./ron1n scan
./ron1n connect
./ron1n status
./ron1n restart
./ron1n logs
./ron1n update
```

## Startup diagnostics

If ron1n cannot start the HTTP server, it now automatically displays the recent systemd journal.

It also checks:

```bash
ss -ltnp
```

for a listener on TCP/8080.

Known legacy services are migrated automatically:

```text
ps4-900-host.service
ps4-jailbreak-host.service
```

An unrelated application using port 8080 is never killed automatically.

## Stable bookmark address

Reserve the Arch host's IP in your router's DHCP settings.

For the machine shown during development:

```text
192.168.1.192
```

the bookmark would be:

```text
http://192.168.1.192:8080/
```

If that address changes, the PS4 bookmark will point to the old host.

## Attribution

`ron1n` is the Stingray Labs orchestration/hosting wrapper.

PSFree, Lapse, GoldHEN, and other upstream jailbreak components remain the work of their respective authors and retain their own licensing and attribution requirements.

---

**ron1n**  
Built by **0xb0rn3 / Stingray Labs**
