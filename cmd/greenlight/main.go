package main

import (
	"context"
	"log"

	"github.com/alecthomas/kong"
	"github.com/fujiwara/greenlight"
)

var Version = "dev"

func main() {
	ctx := context.TODO()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	greenlight.Version = Version
	var cli greenlight.CLI
	kong.Parse(&cli)
	return greenlight.Run(ctx, &cli)
}
