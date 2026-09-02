package main

import (
	"flag"
	"fmt"
	"os"

	"go-web-template/internal/config"
	"go-web-template/internal/platform/logging"
	"go-web-template/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	command := flag.String("command", "up", "migration command: up, down, or version")
	steps := flag.Int("steps", 1, "number of down migrations")
	configFile := flag.String("config", os.Getenv("CONFIG_FILE"), "configuration file path")
	flag.Parse()

	cfg, err := config.Load(*configFile)
	if err != nil {
		return err
	}
	logger, closer, err := logging.New(cfg.Log.File, cfg.Log.Level)
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer.Close()
	}
	version, dirty, err := storage.Migrate(cfg, logger, *command, *steps)
	if err != nil {
		return err
	}
	fmt.Printf("version=%d dirty=%t\n", version, dirty)
	return nil
}
