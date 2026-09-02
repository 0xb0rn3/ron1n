package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/0xb0rn3/ron1n/internal/platform"
	"github.com/0xb0rn3/ron1n/internal/relay"
	"github.com/0xb0rn3/ron1n/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	switch args[0] {
	case "version", "--version", "-V":
		fmt.Println(version.Version)
		return 0
	case "help", "--help", "-h":
		usage()
		return 0
	case "provision":
		return provision(args[1:])
	case "serve":
		return serve(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "ron1n-relay: unknown command %q\n", args[0])
		usage()
		return 2
	}
}

func usage() {
	fmt.Printf(`ron1n-relay %s — self-hostable cross-network static-content relay

Usage:
  ron1n-relay provision --host HOST --token-out FILE [--credentials FILE]
  ron1n-relay serve --external-url https://relay.example [options]
  ron1n-relay version

Production may terminate TLS directly with --tls-cert/--tls-key, or bind the relay
to loopback behind a trusted HTTPS reverse proxy. Public plaintext is refused.
`, version.Version)
}

func provision(args []string) int {
	paths, err := platform.UserPaths()
	if err != nil {
		return fail(err)
	}
	flags := flag.NewFlagSet("provision", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	credentials := flags.String("credentials", filepath.Join(paths.ConfigDir, "relay-hosts.json"), "relay credential database")
	hostID := flags.String("host", "", "host ID")
	tokenOut := flags.String("token-out", "", "write the one-time plaintext host token here")
	rotate := flags.Bool("rotate", false, "rotate an existing host token")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *hostID == "" || *tokenOut == "" {
		fmt.Fprintln(os.Stderr, "provision requires --host and --token-out")
		return 2
	}
	token, err := relay.Provision(*credentials, *hostID, *rotate)
	if err != nil {
		return fail(err)
	}
	if err := relay.WriteToken(*tokenOut, token); err != nil {
		return fail(err)
	}
	fmt.Printf("Provisioned host %s\nCredential database: %s\nHost token file: %s\n", *hostID, *credentials, *tokenOut)
	fmt.Println("The relay stores only the token hash. Transfer the token file to its host securely.")
	return 0
}

func serve(args []string) int {
	paths, err := platform.UserPaths()
	if err != nil {
		return fail(err)
	}
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	listen := flags.String("listen", "127.0.0.1:9090", "listen address")
	externalURL := flags.String("external-url", "", "browser-visible relay URL")
	credentials := flags.String("credentials", filepath.Join(paths.ConfigDir, "relay-hosts.json"), "relay credential database")
	tlsCert := flags.String("tls-cert", "", "TLS certificate file")
	tlsKey := flags.String("tls-key", "", "TLS private key file")
	allowInsecure := flags.Bool("allow-insecure-public", false, "development only: permit plaintext public mode")
	maxResponse := flags.Int64("max-response-bytes", relay.DefaultMaxResponseBytes, "maximum one response body")
	rate := flags.Int("rate-per-minute", 180, "browser requests per source IP per minute")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *externalURL == "" {
		fmt.Fprintln(os.Stderr, "serve requires --external-url")
		return 2
	}
	if err := relay.ValidateListenerSecurity(*listen, *externalURL, *tlsCert, *tlsKey, *allowInsecure); err != nil {
		return fail(err)
	}
	auth, err := relay.LoadAuthenticator(*credentials)
	if err != nil {
		return fail(err)
	}
	handler, err := relay.NewHTTPServer(relay.ServerConfig{
		ExternalURL:      *externalURL,
		MaxResponseBytes: *maxResponse,
		RatePerMinute:    *rate,
	}, auth, relay.NewBroker())
	if err != nil {
		return fail(err)
	}
	server := &http.Server{
		Addr:              *listen,
		Handler:           handler.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("ron1n-relay %s listening on %s for %s\n", version.Version, *listen, *externalURL)
		if *tlsCert != "" {
			errCh <- server.ListenAndServeTLS(*tlsCert, *tlsKey)
		} else {
			errCh <- server.ListenAndServe()
		}
	}()
	select {
	case <-ctx.Done():
		shutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if err := server.Shutdown(shutdown); err != nil {
			return fail(err)
		}
		return 0
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return 0
		}
		return fail(err)
	}
}

func fail(err error) int {
	fmt.Fprintf(os.Stderr, "ron1n-relay: %v\n", err)
	return 1
}
