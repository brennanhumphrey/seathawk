package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/brennanhumphrey/seathawk/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.Execute(ctx); err != nil {
		log.Printf("seathawk: %v\n", err)
		os.Exit(1)
	}
}
