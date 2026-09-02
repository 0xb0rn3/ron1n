package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/0xb0rn3/ron1n/internal/app"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(app.New(os.Stdout, os.Stderr).Run(ctx, os.Args[1:]))
}
