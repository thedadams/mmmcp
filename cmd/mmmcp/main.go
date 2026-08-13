package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stderr, defaultDependencies()); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		_, _ = fmt.Fprintf(os.Stderr, "mmmcp: %v\n", err)
		os.Exit(1)
	}
}
