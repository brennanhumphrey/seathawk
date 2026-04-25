package main

import (
	"context"
	"log"
	"os"

	"github.com/brennanhumphrey/seathawk/internal/cli"
)

func main() {
	ctx := context.Background()

	if err := cli.Execute(ctx); err != nil {
		log.Printf("seathawk: %v\n", err)
		os.Exit(1)
	}
}
