package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xb0rn3/ron1n/internal/catalog"
	"github.com/0xb0rn3/ron1n/internal/config"
	"github.com/0xb0rn3/ron1n/internal/content"
	"github.com/0xb0rn3/ron1n/internal/host"
	"github.com/0xb0rn3/ron1n/internal/platform"
	"github.com/0xb0rn3/ron1n/internal/relay"
	"github.com/0xb0rn3/ron1n/internal/state"
	"github.com/0xb0rn3/ron1n/internal/version"
)

type App struct {
	Stdout io.Writer
	Stderr io.Writer
	Now    func() time.Time
}

func New(stdout, stderr io.Writer) *App {
	return &App{Stdout: stdout, Stderr: stderr, Now: time.Now}
}

func (app *App) Run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return app.install(ctx, nil)
	}
	switch args[0] {
	case "version", "--version", "-V":
		fmt.Fprintln(app.Stdout, version.Version)
		return 0
	case "help", "--help", "-h":
		app.usage(app.Stdout)
		return 0
	case "install":
		return app.install(ctx, args[1:])
	case "serve":
		return app.serve(ctx, args[1:])
	case "status":
		return app.status(args[1:])
	case "wait":
		return app.wait(ctx, args[1:], false)
	case "watch":
		return app.wait(ctx, args[1:], true)
	case "connect":
		return app.connect(args[1:])
	case "restart":
		return app.restart(args[1:])
	case "logs":
		return app.logs(ctx, args[1:])
	case "update":
		return app.sync(ctx, args[1:], true)
	case "content":
		return app.content(ctx, args[1:])
	case "keys":
		return app.keys(args[1:])
	case "relay":
		return app.relay(ctx, args[1:])
	case "sources":
		return app.sources(args[1:])
	case "doctor":
		return app.doctor(args[1:])
	case "uninstall":
		return app.uninstall(args[1:])
	default:
		fmt.Fprintf(app.Stderr, "ron1n: unknown command %q\n\n", args[0])
		app.usage(app.Stderr)
		return 2
	}
}

func (app *App) usage(output io.Writer) {
	fmt.Fprintf(output, `ron1n %s — verified PS4 host and cross-network relay

Usage:
  ron1n install [--service]              Initialize, import pinned content, and optionally autostart
  ron1n serve                            Run the local byte-exact HTTP host
  ron1n status [--json]                  Show service, host, bundle, and recent console state
  ron1n wait | watch                     Wait for or continuously show browser activity
  ron1n connect                          Show local and remote connection instructions
  ron1n restart | logs                   Manage or inspect the installed host
  ron1n update                           Import the current upstream commit as a new signed bundle
  ron1n content sync|build|verify         Manage allowlisted content bundles
  ron1n keys generate                    Generate an Ed25519 bundle-signing key pair
  ron1n relay connect|session|revoke      Run an agent or manage browser sessions
  ron1n sources [--json]                 Show audited related host/delivery repositories
  ron1n doctor                           Validate configuration, signatures, bytes, and service health
  ron1n uninstall                        Remove autostart integration; preserve content and keys
  ron1n version                          Print the exact product version

Remote mode is opt-in. It forwards only allowlisted GET/HEAD content and has no shell,
arbitrary proxy, scan, or upload-and-execute capability.
`, version.Version)
}

func (app *App) install(ctx context.Context, args []string) int {
	flags := app.flags("install")
	configPath := flags.String("config", "", "configuration file")
	repository := flags.String("repo", envOr("RON1N_REPO_URL", content.DefaultPSFreeRepo), "GitHub content repository")
	revision := flags.String("ref", content.DefaultPSFreeRevision, "full commit, tag, or branch to resolve")
	archiveSHA := flags.String("archive-sha256", "", "optional expected source archive SHA-256")
	service := flags.Bool("service", false, "install and start user autostart integration")
	noFetch := flags.Bool("no-fetch", false, "initialize only; do not download content")
	if !app.parse(flags, args) {
		return 2
	}
	paths, cfg, name, err := app.loadConfig(*configPath, true)
	if err != nil {
		return app.fail(err)
	}
	if !*noFetch {
		privatePath, publicPath, err := ensureSigningKeys(paths)
		if err != nil {
			return app.fail(err)
		}
		private, _, err := content.LoadPrivateKey(privatePath)
		if err != nil {
			return app.fail(err)
		}
		result, err := content.ImportGitHub(ctx, content.ImportOptions{
			Repository:      *repository,
			Revision:        *revision,
			DestinationRoot: filepath.Join(paths.DataDir, "content"),
			Profile:         "psfree-lapse-900",
			ArchiveSHA256:   *archiveSHA,
			SigningKey:      private,
			Now:             app.Now(),
		})
		if err != nil {
			return app.fail(err)
		}
		cfg.ContentDir = result.Directory
		cfg.Manifest = result.ManifestPath
		fmt.Fprintf(app.Stdout, "Imported %s at %s\n", *repository, result.Revision)
		fmt.Fprintf(app.Stdout, "Archive SHA-256: %s\n", result.ArchiveSHA256)
		fmt.Fprintf(app.Stdout, "Bundle signing public key: %s\n", publicPath)
	}
	if err := config.Save(name, cfg); err != nil {
		return app.fail(err)
	}
	if *service {
		executable, err := os.Executable()
		if err != nil {
			return app.fail(err)
		}
		installed, err := platform.InstallService(paths, executable, name)
		if err != nil {
			return app.fail(err)
		}
		fmt.Fprintf(app.Stdout, "Autostart installed: %s\n", installed)
	}
	fmt.Fprintf(app.Stdout, "ron1n %s initialized. Configuration: %s\n", version.Version, name)
	if !*service {
		fmt.Fprintln(app.Stdout, "Start the host with: ron1n serve")
	}
	app.printLocalURLs(cfg.Listen)
	return 0
}

func (app *App) serve(ctx context.Context, args []string) int {
	flags := app.flags("serve")
	configPath := flags.String("config", "", "configuration file")
	listen := flags.String("listen", "", "override listen address")
	root := flags.String("root", "", "override content directory")
	manifest := flags.String("manifest", "", "override manifest path")
	trustedKey := flags.String("trusted-key", "", "trusted Ed25519 public key")
	allowUnsigned := flags.Bool("allow-unsigned", false, "development only: permit unsigned content")
	if !app.parse(flags, args) {
		return 2
	}
	paths, cfg, _, err := app.loadConfig(*configPath, false)
	if err != nil {
		return app.fail(err)
	}
	if *listen != "" {
		cfg.Listen = *listen
	}
	if *root != "" {
		cfg.ContentDir = *root
	}
	if *manifest != "" {
		cfg.Manifest = *manifest
	}
	contentHost, err := app.verifiedHost(paths, cfg, *trustedKey, *allowUnsigned)
	if err != nil {
		return app.fail(err)
	}
	httpServer := &http.Server{
		Addr:              cfg.Listen,
		Handler:           contentHost,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	errCh := make(chan error, 1)
	go func() {
		fmt.Fprintf(app.Stdout, "ron1n %s serving verified bundle %s\n", version.Version, contentHost.Manifest.BundleID)
		app.printLocalURLs(cfg.Listen)
		errCh <- httpServer.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdown); err != nil {
			return app.fail(err)
		}
		return 0
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return 0
		}
		return app.fail(err)
	}
}

func (app *App) status(args []string) int {
	flags := app.flags("status")
	configPath := flags.String("config", "", "configuration file")
	jsonOutput := flags.Bool("json", false, "emit JSON")
	if !app.parse(flags, args) {
		return 2
	}
	_, cfg, _, err := app.loadConfig(*configPath, false)
	if err != nil {
		return app.fail(err)
	}
	type output struct {
		Version  string             `json:"version"`
		Service  string             `json:"service"`
		Listen   string             `json:"listen"`
		Manifest string             `json:"manifest"`
		Health   string             `json:"health"`
		Recent   bool               `json:"recent"`
		Console  state.ConsoleState `json:"console,omitempty"`
	}
	result := output{Version: version.Version, Service: platform.ServiceStatus(), Listen: cfg.Listen, Manifest: cfg.Manifest, Health: app.health(cfg.Listen)}
	console, recent, readErr := state.New(cfg.StateDir).Recent(time.Duration(cfg.RecentHTTP)*time.Second, app.Now())
	if readErr == nil {
		result.Console = console
		result.Recent = recent
	}
	if *jsonOutput {
		b, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(app.Stdout, string(b))
		return 0
	}
	fmt.Fprintf(app.Stdout, "ron1n %s\nService: %s\nListen: %s\nHealth: %s\nManifest: %s\n", result.Version, result.Service, result.Listen, result.Health, result.Manifest)
	if result.Console.Timestamp > 0 {
		age := app.Now().Sub(time.Unix(result.Console.Timestamp, 0)).Round(time.Second)
		fmt.Fprintf(app.Stdout, "Console: stage=%s phase=%s transport=%s age=%s recent=%t\n", result.Console.Stage, result.Console.Phase, result.Console.Transport, age, result.Recent)
	} else {
		fmt.Fprintln(app.Stdout, "Console: no PlayStation browser activity recorded")
	}
	return 0
}

func (app *App) wait(ctx context.Context, args []string, continuous bool) int {
	name := "wait"
	if continuous {
		name = "watch"
	}
	flags := app.flags(name)
	configPath := flags.String("config", "", "configuration file")
	jsonOutput := flags.Bool("json", false, "emit JSON Lines")
	if !app.parse(flags, args) {
		return 2
	}
	_, cfg, _, err := app.loadConfig(*configPath, false)
	if err != nil {
		return app.fail(err)
	}
	store := state.New(cfg.StateDir)
	last := ""
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	fmt.Fprintln(app.Stdout, "Waiting for PlayStation browser activity; Ctrl+C to stop.")
	for {
		console, err := store.Console()
		fingerprint := fmt.Sprintf("%d:%s:%s", console.Timestamp, console.RequestID, console.Phase)
		if err == nil && fingerprint != last {
			last = fingerprint
			if *jsonOutput {
				b, _ := json.Marshal(console)
				fmt.Fprintln(app.Stdout, string(b))
			} else {
				fmt.Fprintf(app.Stdout, "%s  %-28s %-20s %s\n", time.Unix(console.Timestamp, 0).Format(time.RFC3339), console.Stage, console.Phase, console.Transport)
			}
			if !continuous {
				return 0
			}
		}
		select {
		case <-ctx.Done():
			return 0
		case <-ticker.C:
		}
	}
}

func (app *App) connect(args []string) int {
	flags := app.flags("connect")
	configPath := flags.String("config", "", "configuration file")
	if !app.parse(flags, args) {
		return 2
	}
	_, cfg, _, err := app.loadConfig(*configPath, false)
	if err != nil {
		return app.fail(err)
	}
	fmt.Fprintln(app.Stdout, "Local mode (same LAN, offline-capable):")
	app.printLocalURLs(cfg.Listen)
	fmt.Fprintln(app.Stdout, "  Open /cache.html once, wait for cache completion, then bookmark the session root.")
	fmt.Fprintln(app.Stdout, "\nRemote mode (different networks):")
	fmt.Fprintln(app.Stdout, "  1. Start: ron1n relay connect")
	fmt.Fprintln(app.Stdout, "  2. Create: ron1n relay session --ttl 30m")
	fmt.Fprintln(app.Stdout, "  3. Open the printed HTTPS /cache.html URL on the console.")
	fmt.Fprintln(app.Stdout, "Remote sessions are capability URLs; do not post them publicly.")
	return 0
}

func (app *App) restart(args []string) int {
	flags := app.flags("restart")
	if !app.parse(flags, args) {
		return 2
	}
	if err := platform.RestartService(); err != nil {
		return app.fail(err)
	}
	fmt.Fprintln(app.Stdout, "ron1n background host restarted")
	return 0
}

func (app *App) logs(ctx context.Context, args []string) int {
	flags := app.flags("logs")
	configPath := flags.String("config", "", "configuration file")
	follow := flags.Bool("follow", true, "follow new events")
	if !app.parse(flags, args) {
		return 2
	}
	_, cfg, _, err := app.loadConfig(*configPath, false)
	if err != nil {
		return app.fail(err)
	}
	name := filepath.Join(cfg.StateDir, "events.log")
	position := int64(0)
	for {
		file, err := os.Open(name)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return app.fail(err)
			}
		} else {
			info, _ := file.Stat()
			if info != nil && position > info.Size() {
				position = 0
			}
			_, _ = file.Seek(position, io.SeekStart)
			n, _ := io.Copy(app.Stdout, file)
			position += n
			file.Close()
		}
		if !*follow {
			return 0
		}
		select {
		case <-ctx.Done():
			return 0
		case <-time.After(time.Second):
		}
	}
}

func (app *App) content(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(app.Stderr, "usage: ron1n content sync|build|verify")
		return 2
	}
	switch args[0] {
	case "sync":
		return app.sync(ctx, args[1:], false)
	case "build":
		return app.buildManifest(args[1:])
	case "verify":
		return app.verifyManifest(args[1:])
	default:
		fmt.Fprintf(app.Stderr, "unknown content command %q\n", args[0])
		return 2
	}
}

func (app *App) sync(ctx context.Context, args []string, updateAlias bool) int {
	flags := app.flags("content sync")
	configPath := flags.String("config", "", "configuration file")
	repository := flags.String("repo", envOr("RON1N_REPO_URL", content.DefaultPSFreeRepo), "GitHub content repository")
	defaultRef := content.DefaultPSFreeRevision
	if updateAlias {
		defaultRef = "main"
	}
	revision := flags.String("ref", defaultRef, "commit, tag, or branch to resolve")
	archiveSHA := flags.String("archive-sha256", "", "optional expected source archive SHA-256")
	profile := flags.String("profile", "psfree-lapse-900", "content compatibility profile")
	restart := flags.Bool("restart", updateAlias, "restart the installed service after activation")
	if !app.parse(flags, args) {
		return 2
	}
	paths, cfg, name, err := app.loadConfig(*configPath, true)
	if err != nil {
		return app.fail(err)
	}
	privatePath, _, err := ensureSigningKeys(paths)
	if err != nil {
		return app.fail(err)
	}
	private, _, err := content.LoadPrivateKey(privatePath)
	if err != nil {
		return app.fail(err)
	}
	result, err := content.ImportGitHub(ctx, content.ImportOptions{
		Repository:      *repository,
		Revision:        *revision,
		DestinationRoot: filepath.Join(paths.DataDir, "content"),
		Profile:         *profile,
		ArchiveSHA256:   *archiveSHA,
		SigningKey:      private,
		Now:             app.Now(),
	})
	if err != nil {
		return app.fail(err)
	}
	cfg.ContentDir = result.Directory
	cfg.Manifest = result.ManifestPath
	if err := config.Save(name, cfg); err != nil {
		return app.fail(err)
	}
	fmt.Fprintf(app.Stdout, "Activated bundle %s from %s at %s\n", result.Envelope.Manifest.BundleID, *repository, result.Revision)
	if *restart && platform.ServiceStatus() != "stopped-or-unavailable" {
		if err := platform.RestartService(); err != nil {
			return app.fail(err)
		}
		fmt.Fprintln(app.Stdout, "Background host restarted")
	}
	return 0
}

func (app *App) buildManifest(args []string) int {
	flags := app.flags("content build")
	root := flags.String("root", ".", "content root")
	out := flags.String("out", "ron1n-manifest.json", "manifest output")
	profile := flags.String("profile", "static", "content profile")
	source := flags.String("source", "local", "provenance source")
	revision := flags.String("revision", "local", "provenance revision")
	privateKey := flags.String("private-key", "", "Ed25519 signing key")
	if !app.parse(flags, args) {
		return 2
	}
	manifest, err := content.Build(*root, *profile, *source, *revision, app.Now(), time.Time{})
	if err != nil {
		return app.fail(err)
	}
	envelope := content.Envelope{Manifest: manifest}
	if *privateKey != "" {
		private, _, err := content.LoadPrivateKey(*privateKey)
		if err != nil {
			return app.fail(err)
		}
		envelope, err = content.Sign(manifest, private)
		if err != nil {
			return app.fail(err)
		}
	}
	if err := content.Save(*out, envelope); err != nil {
		return app.fail(err)
	}
	fmt.Fprintf(app.Stdout, "Wrote bundle %s to %s\n", manifest.BundleID, *out)
	return 0
}

func (app *App) verifyManifest(args []string) int {
	flags := app.flags("content verify")
	root := flags.String("root", ".", "content root")
	manifestPath := flags.String("manifest", "ron1n-manifest.json", "manifest path")
	publicKey := flags.String("public-key", "", "trusted Ed25519 public key")
	allowUnsigned := flags.Bool("allow-unsigned", false, "permit unsigned local manifest")
	if !app.parse(flags, args) {
		return 2
	}
	envelope, err := content.Load(*manifestPath)
	if err != nil {
		return app.fail(err)
	}
	if err := verifyEnvelope(envelope, *publicKey, *allowUnsigned); err != nil {
		return app.fail(err)
	}
	if err := content.VerifyFiles(*root, envelope.Manifest, app.Now()); err != nil {
		return app.fail(err)
	}
	fmt.Fprintf(app.Stdout, "Verified %d files in bundle %s\n", len(envelope.Manifest.Files), envelope.Manifest.BundleID)
	return 0
}

func (app *App) keys(args []string) int {
	if len(args) == 0 || args[0] != "generate" {
		fmt.Fprintln(app.Stderr, "usage: ron1n keys generate [--private FILE --public FILE]")
		return 2
	}
	flags := app.flags("keys generate")
	privatePath := flags.String("private", "ron1n-signing.key", "private key output")
	publicPath := flags.String("public", "ron1n-signing.pub", "public key output")
	force := flags.Bool("force", false, "replace an existing key pair")
	if !app.parse(flags, args[1:]) {
		return 2
	}
	if !*force && (exists(*privatePath) || exists(*publicPath)) {
		return app.fail(errors.New("key output exists; refusing replacement without --force"))
	}
	id, err := content.GenerateKeyPair(*privatePath, *publicPath)
	if err != nil {
		return app.fail(err)
	}
	fmt.Fprintf(app.Stdout, "Generated Ed25519 key %s\nPrivate: %s\nPublic: %s\n", id, *privatePath, *publicPath)
	return 0
}

func (app *App) relay(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(app.Stderr, "usage: ron1n relay configure|connect|session|revoke")
		return 2
	}
	switch args[0] {
	case "configure":
		return app.relayConfigure(args[1:])
	case "connect":
		return app.relayConnect(ctx, args[1:])
	case "session":
		return app.relaySession(ctx, args[1:])
	case "revoke":
		return app.relayRevoke(ctx, args[1:])
	default:
		fmt.Fprintf(app.Stderr, "unknown relay command %q\n", args[0])
		return 2
	}
}

func (app *App) relayConfigure(args []string) int {
	flags := app.flags("relay configure")
	configPath := flags.String("config", "", "configuration file")
	relayURL := flags.String("url", "", "relay base URL")
	hostID := flags.String("host-id", "", "provisioned relay host ID")
	tokenFile := flags.String("token-file", "", "relay token file")
	workers := flags.Int("workers", 4, "concurrent outbound workers")
	allowHTTP := flags.Bool("allow-http", false, "development only: permit plaintext relay URL")
	if !app.parse(flags, args) {
		return 2
	}
	if *relayURL == "" || *hostID == "" || *tokenFile == "" {
		fmt.Fprintln(app.Stderr, "relay configure requires --url, --host-id, and --token-file")
		return 2
	}
	if _, err := relay.ReadToken(*tokenFile); err != nil {
		return app.fail(fmt.Errorf("read relay token: %w", err))
	}
	if _, err := relay.NewAgent(relay.AgentConfig{
		RelayURL:  *relayURL,
		HostID:    *hostID,
		Token:     strings.Repeat("x", 32),
		Workers:   *workers,
		AllowHTTP: *allowHTTP,
	}, &host.Server{}); err != nil {
		return app.fail(err)
	}
	_, cfg, name, err := app.loadConfig(*configPath, true)
	if err != nil {
		return app.fail(err)
	}
	applyRelayFlags(&cfg, *relayURL, *hostID, *tokenFile, *workers)
	if err := config.Save(name, cfg); err != nil {
		return app.fail(err)
	}
	fmt.Fprintf(app.Stdout, "Saved relay configuration for %s in %s\n", *hostID, name)
	fmt.Fprintln(app.Stdout, "The token itself remains only in its restricted token file.")
	return 0
}

func (app *App) relayConnect(ctx context.Context, args []string) int {
	flags := app.flags("relay connect")
	configPath := flags.String("config", "", "configuration file")
	relayURL := flags.String("url", "", "relay base URL")
	hostID := flags.String("host-id", "", "provisioned relay host ID")
	tokenFile := flags.String("token-file", "", "relay token file")
	workers := flags.Int("workers", 0, "concurrent outbound workers")
	allowHTTP := flags.Bool("allow-http", false, "development only: permit plaintext non-production relay")
	if !app.parse(flags, args) {
		return 2
	}
	paths, cfg, _, err := app.loadConfig(*configPath, false)
	if err != nil {
		return app.fail(err)
	}
	applyRelayFlags(&cfg, *relayURL, *hostID, *tokenFile, *workers)
	token, err := relay.ReadToken(cfg.Relay.TokenFile)
	if err != nil {
		return app.fail(err)
	}
	contentHost, err := app.verifiedHost(paths, cfg, "", false)
	if err != nil {
		return app.fail(err)
	}
	agent, err := relay.NewAgent(relay.AgentConfig{
		RelayURL:  cfg.Relay.URL,
		HostID:    cfg.Relay.HostID,
		Token:     token,
		Workers:   cfg.Relay.Workers,
		AllowHTTP: *allowHTTP,
	}, contentHost)
	if err != nil {
		return app.fail(err)
	}
	fmt.Fprintf(app.Stdout, "ron1n outbound agent connected as %s with %d workers\n", cfg.Relay.HostID, cfg.Relay.Workers)
	if err := agent.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return app.fail(err)
	}
	return 0
}

func (app *App) relaySession(ctx context.Context, args []string) int {
	flags := app.flags("relay session")
	configPath := flags.String("config", "", "configuration file")
	relayURL := flags.String("url", "", "relay base URL")
	hostID := flags.String("host-id", "", "provisioned relay host ID")
	tokenFile := flags.String("token-file", "", "relay token file")
	ttl := flags.Duration("ttl", 30*time.Minute, "session lifetime (1m-24h)")
	allowHTTP := flags.Bool("allow-http", false, "development only: permit plaintext relay")
	if !app.parse(flags, args) {
		return 2
	}
	_, cfg, _, err := app.loadConfig(*configPath, false)
	if err != nil {
		return app.fail(err)
	}
	applyRelayFlags(&cfg, *relayURL, *hostID, *tokenFile, 0)
	token, err := relay.ReadToken(cfg.Relay.TokenFile)
	if err != nil {
		return app.fail(err)
	}
	envelope, err := content.Load(cfg.Manifest)
	if err != nil {
		return app.fail(err)
	}
	result, err := relay.CreateRemoteSession(ctx, cfg.Relay.URL, cfg.Relay.HostID, token, envelope.Manifest.BundleID, *ttl, *allowHTTP, nil)
	if err != nil {
		return app.fail(err)
	}
	fmt.Fprintf(app.Stdout, "Session ID: %s\nSession expires: %s\nRoot: %s\nOffline cache: %scache.html\n", result.ID, result.ExpiresAt, result.URL, result.URL)
	fmt.Fprintf(app.Stdout, "Treat this capability URL as a secret. Revoke it with: ron1n relay revoke --session %s\n", result.ID)
	return 0
}

func (app *App) relayRevoke(ctx context.Context, args []string) int {
	flags := app.flags("relay revoke")
	configPath := flags.String("config", "", "configuration file")
	relayURL := flags.String("url", "", "relay base URL")
	hostID := flags.String("host-id", "", "provisioned relay host ID")
	tokenFile := flags.String("token-file", "", "relay token file")
	sessionID := flags.String("session", "", "session ID returned at creation")
	allowHTTP := flags.Bool("allow-http", false, "development only: permit plaintext relay")
	if !app.parse(flags, args) {
		return 2
	}
	if *sessionID == "" {
		fmt.Fprintln(app.Stderr, "relay revoke requires --session")
		return 2
	}
	_, cfg, _, err := app.loadConfig(*configPath, false)
	if err != nil {
		return app.fail(err)
	}
	applyRelayFlags(&cfg, *relayURL, *hostID, *tokenFile, 0)
	token, err := relay.ReadToken(cfg.Relay.TokenFile)
	if err != nil {
		return app.fail(err)
	}
	if err := relay.RevokeRemoteSession(ctx, cfg.Relay.URL, cfg.Relay.HostID, token, *sessionID, *allowHTTP, nil); err != nil {
		return app.fail(err)
	}
	fmt.Fprintf(app.Stdout, "Revoked relay session %s\n", *sessionID)
	return 0
}

func (app *App) sources(args []string) int {
	flags := app.flags("sources")
	jsonOutput := flags.Bool("json", false, "emit JSON")
	if !app.parse(flags, args) {
		return 2
	}
	if *jsonOutput {
		b, _ := json.MarshalIndent(catalog.Sources, "", "  ")
		fmt.Fprintln(app.Stdout, string(b))
		return 0
	}
	for _, source := range catalog.Sources {
		fmt.Fprintf(app.Stdout, "%s\n  %s | %s\n  %s\n  Policy: %s\n", source.Name, source.Category, source.License, source.Role, source.Policy)
	}
	return 0
}

func (app *App) doctor(args []string) int {
	flags := app.flags("doctor")
	configPath := flags.String("config", "", "configuration file")
	if !app.parse(flags, args) {
		return 2
	}
	paths, cfg, name, err := app.loadConfig(*configPath, false)
	if err != nil {
		return app.fail(err)
	}
	fmt.Fprintf(app.Stdout, "[ok] config: %s\n", name)
	if _, err := app.verifiedHost(paths, cfg, "", false); err != nil {
		return app.fail(err)
	}
	fmt.Fprintln(app.Stdout, "[ok] manifest signature and every content byte")
	fmt.Fprintf(app.Stdout, "[%s] local health\n", app.health(cfg.Listen))
	fmt.Fprintf(app.Stdout, "[%s] background integration\n", platform.ServiceStatus())
	if cfg.Relay.URL == "" || cfg.Relay.HostID == "" {
		fmt.Fprintln(app.Stdout, "[optional] relay is not configured")
	} else if _, err := relay.ReadToken(cfg.Relay.TokenFile); err != nil {
		fmt.Fprintln(app.Stdout, "[warn] relay configured but token file is unreadable")
	} else {
		fmt.Fprintln(app.Stdout, "[ok] relay configuration and token file")
	}
	return 0
}

func (app *App) uninstall(args []string) int {
	flags := app.flags("uninstall")
	configPath := flags.String("config", "", "configuration file")
	if !app.parse(flags, args) {
		return 2
	}
	paths, _, _, err := app.loadConfig(*configPath, false)
	if err != nil {
		return app.fail(err)
	}
	if err := platform.RemoveService(paths); err != nil {
		return app.fail(err)
	}
	fmt.Fprintln(app.Stdout, "Removed ron1n autostart integration. Content, state, configuration, and signing keys were preserved.")
	return 0
}

func (app *App) verifiedHost(paths platform.Paths, cfg config.Config, trustedKey string, allowUnsigned bool) (*host.Server, error) {
	envelope, err := content.Load(cfg.Manifest)
	if err != nil {
		return nil, fmt.Errorf("load content manifest: %w", err)
	}
	if trustedKey == "" {
		trustedKey = filepath.Join(paths.ConfigDir, "content-signing.pub")
	}
	if err := verifyEnvelope(envelope, trustedKey, allowUnsigned); err != nil {
		return nil, err
	}
	if err := content.VerifyFiles(cfg.ContentDir, envelope.Manifest, app.Now()); err != nil {
		return nil, err
	}
	return &host.Server{Root: cfg.ContentDir, Manifest: envelope.Manifest, State: state.New(cfg.StateDir)}, nil
}

func verifyEnvelope(envelope content.Envelope, publicKeyPath string, allowUnsigned bool) error {
	if envelope.Signature == "" {
		if allowUnsigned {
			return nil
		}
		return errors.New("content manifest is unsigned; sign it or use --allow-unsigned only for development")
	}
	if publicKeyPath == "" {
		return errors.New("a trusted public key is required for signed content")
	}
	public, _, err := content.LoadPublicKey(publicKeyPath)
	if err != nil {
		return fmt.Errorf("load trusted public key: %w", err)
	}
	return content.VerifySignature(envelope, public)
}

func ensureSigningKeys(paths platform.Paths) (string, string, error) {
	privatePath := filepath.Join(paths.ConfigDir, "content-signing.key")
	publicPath := filepath.Join(paths.ConfigDir, "content-signing.pub")
	privateExists := exists(privatePath)
	publicExists := exists(publicPath)
	if privateExists != publicExists {
		return "", "", errors.New("only one content signing key file exists; restore the pair instead of rotating trust implicitly")
	}
	if !privateExists {
		if _, err := content.GenerateKeyPair(privatePath, publicPath); err != nil {
			return "", "", err
		}
	}
	return privatePath, publicPath, nil
}

func (app *App) loadConfig(override string, ensure bool) (platform.Paths, config.Config, string, error) {
	paths, err := platform.UserPaths()
	if err != nil {
		return platform.Paths{}, config.Config{}, "", err
	}
	if ensure {
		if err := paths.Ensure(); err != nil {
			return platform.Paths{}, config.Config{}, "", err
		}
	}
	name := override
	if name == "" {
		name = config.File(paths)
	}
	cfg, err := config.Load(name, config.Defaults(paths))
	return paths, cfg, name, err
}

func (app *App) health(listen string) string {
	_, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "invalid-listen"
	}
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://127.0.0.1:" + port + "/_ron1n/health")
	if err != nil {
		return "unreachable"
	}
	response.Body.Close()
	if response.StatusCode == http.StatusOK {
		return "ok"
	}
	return "unhealthy"
}

func (app *App) printLocalURLs(listen string) {
	_, port, err := net.SplitHostPort(listen)
	if err != nil {
		return
	}
	ip := firstLANIPv4()
	if ip == "" {
		ip = "HOST-IP"
	}
	fmt.Fprintf(app.Stdout, "Local root: http://%s:%s/\nOffline cache: http://%s:%s/cache.html\n", ip, port, ip, port)
}

func firstLANIPv4() string {
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := iface.Addrs()
		for _, address := range addresses {
			var ip net.IP
			switch value := address.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			if value := ip.To4(); value != nil && !value.IsLoopback() {
				return value.String()
			}
		}
	}
	return ""
}

func applyRelayFlags(cfg *config.Config, relayURL, hostID, tokenFile string, workers int) {
	if relayURL != "" {
		cfg.Relay.URL = relayURL
	}
	if hostID != "" {
		cfg.Relay.HostID = hostID
	}
	if tokenFile != "" {
		cfg.Relay.TokenFile = tokenFile
	}
	if workers != 0 {
		cfg.Relay.Workers = workers
	}
}

func (app *App) flags(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(app.Stderr)
	return flags
}

func (app *App) parse(flags *flag.FlagSet, args []string) bool {
	if err := flags.Parse(args); err != nil {
		return false
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(app.Stderr, "%s: unexpected arguments: %s\n", flags.Name(), strings.Join(flags.Args(), " "))
		return false
	}
	return true
}

func (app *App) fail(err error) int {
	fmt.Fprintf(app.Stderr, "ron1n: %v\n", err)
	return 1
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func exists(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}
