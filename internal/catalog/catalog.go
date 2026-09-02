package catalog

type Entry struct {
	Name       string
	URL        string
	Category   string
	License    string
	Role       string
	Policy     string
	DefaultRef string
}

var Sources = []Entry{
	{
		Name:       "kmeps4/PSFree",
		URL:        "https://github.com/kmeps4/PSFree",
		Category:   "active-content",
		License:    "AGPL-3.0-or-later; bundled font OFL-1.1; GoldHEN licensing unverified",
		Role:       "Current ron1n PS4 9.00 PSFree/Lapse/AIO/GoldHEN host tree",
		Policy:     "Pinned import with preserved notices; private delivery; operator-signed manifest",
		DefaultRef: "368d82aa40d3017c220757ce315761adb5f06678",
	},
	{
		Name:     "Al-Azif/psfree-lapse",
		URL:      "https://github.com/Al-Azif/psfree-lapse",
		Category: "alternative-content",
		License:  "AGPL-3.0-or-later",
		Role:     "Maintained combined PSFree/Lapse lineage",
		Policy:   "Catalogued, never silently substituted for kmeps4/PSFree",
	},
	{
		Name:     "GoldHEN/GoldHEN",
		URL:      "https://github.com/GoldHEN/GoldHEN",
		Category: "payload-upstream",
		License:  "No repository license detected",
		Role:     "Official GoldHEN release channel",
		Policy:   "Reference/user-supplied only; no public mirroring or authorship claim",
	},
	{
		Name:     "GoldHEN/henloader_lp",
		URL:      "https://github.com/GoldHEN/henloader_lp",
		Category: "loader-reference",
		License:  "MIT",
		Role:     "Current Lapse/Poops loader reference across later firmware",
		Policy:   "Compatibility research only for 0.0.1zoro; no firmware-chain substitution",
	},
	{
		Name:     "Al-Azif/ps4-exploit-host",
		URL:      "https://github.com/Al-Azif/ps4-exploit-host",
		Category: "backend-reference",
		License:  "MIT",
		Role:     "Cross-platform HTTP/DNS host, static build, caching, updater and payload handoff",
		Policy:   "Adopt narrow configuration/cache/error-handling patterns, not unrestricted forwarding",
	},
	{
		Name:     "Al-Azif/exploit-host-dns",
		URL:      "https://github.com/Al-Azif/exploit-host-dns",
		Category: "dns-reference",
		License:  "MIT",
		Role:     "Isolated DNS redirect and update/telemetry blocking component",
		Policy:   "Optional future adapter; never required for remote relay delivery",
	},
	{
		Name:     "ArabPixel/WebKitty",
		URL:      "https://github.com/ArabPixel/WebKitty",
		Category: "host-reference",
		License:  "AGPL-3.0-or-later",
		Role:     "Firmware-aware manifests, auto-detection, payload selection and remote host",
		Policy:   "Adopt profile-specific manifests; keep exploit chains isolated by firmware",
	},
	{
		Name:     "ArabPixel/PET",
		URL:      "https://github.com/ArabPixel/PET",
		Category: "test-reference",
		License:  "AGPL-3.0-or-later",
		Role:     "PSFree Enhanced test repository",
		Policy:   "Research/test corpus only",
	},
	{
		Name:     "ArabPixel/ps4-websrv",
		URL:      "https://github.com/ArabPixel/ps4-websrv",
		Category: "on-console-reference",
		License:  "MIT",
		Role:     "HTTP server running on the console for device-triggered payload loading",
		Policy:   "Catalogued architecture; not bundled into the host/relay rewrite",
	},
	{
		Name:     "muratcesmecioglu/ha-ps4-jb",
		URL:      "https://github.com/muratcesmecioglu/ha-ps4-jb",
		Category: "appliance-reference",
		License:  "GPL-3.0",
		Role:     "Home Assistant/static host with persistent payload management",
		Policy:   "Adopt persistent staged activation; reject unsigned arbitrary admin uploads",
	},
	{
		Name:     "stooged/PS4-Server-900u",
		URL:      "https://github.com/stooged/PS4-Server-900u",
		Category: "usb-appliance-reference",
		License:  "No repository license detected",
		Role:     "PS4 9.00 server with USB control",
		Policy:   "Reference only",
	},
	{
		Name:     "stooged/ESP32-Server-900u",
		URL:      "https://github.com/stooged/ESP32-Server-900u",
		Category: "embedded-reference",
		License:  "No repository license detected",
		Role:     "ESP32 9.00 host with USB emulation",
		Policy:   "Reference only",
	},
	{
		Name:     "Shivelight/pOOBs4-buildroot",
		URL:      "https://github.com/Shivelight/pOOBs4-buildroot",
		Category: "appliance-reference",
		License:  "GPL-2.0",
		Role:     "Small Linux web/USB-emulation appliance",
		Policy:   "Use as deployment-family reference; no copied image or payload",
	},
	{
		Name:     "ionontelodico/GoldHEN-Loader-ESP32",
		URL:      "https://github.com/ionontelodico/GoldHEN-Loader-ESP32",
		Category: "embedded-reference",
		License:  "MIT",
		Role:     "ESP32-S3 direct GoldHEN loader",
		Policy:   "Reference for constrained-device delivery acknowledgements",
	},
	{
		Name:     "frwololo/PS4_PS5-ESP8266-Server",
		URL:      "https://github.com/frwololo/PS4_PS5-ESP8266-Server",
		Category: "embedded-reference",
		License:  "No repository license detected",
		Role:     "Embedded web server, repeater and fake DNS host",
		Policy:   "Reference only",
	},
}
