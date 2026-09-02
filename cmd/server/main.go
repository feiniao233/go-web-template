package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go-web-template/internal/bootstrap"
)

func main() {
	configFile := flag.String("config", os.Getenv("CONFIG_FILE"), "configuration file path")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := bootstrap.Run(ctx, bootstrap.Options{ConfigFile: *configFile}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
