// Package cmd is the catalystcal command line: serve (default), migrate, version.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/catalystcommunity/catalystcal/api/internal/config"
	"github.com/catalystcommunity/catalystcal/api/internal/store"
	log "github.com/sirupsen/logrus"
)

const usage = `catalystcal: a calendaring service

Usage:
  catalystcal [serve]   start the API server (applies migrations first)
  catalystcal migrate   apply the database migrations and exit
  catalystcal version   print the version
  catalystcal help      print this text

Configuration is read from CATALYSTCAL_* environment variables.`

// errShutdown means a signal ended a wait. It maps to a zero exit.
var errShutdown = errors.New("shutdown requested")

// Run runs the command line.
func Run(version string, args []string) error {
	verb := "serve"
	if len(args) > 0 {
		verb = args[0]
	}
	switch verb {
	case "serve":
		err := runServe(version)
		if errors.Is(err, errShutdown) {
			return nil
		}
		return err
	case "migrate":
		return runMigrate()
	case "version", "--version", "-v":
		fmt.Printf("catalystcal %s\n", version)
		return nil
	case "help", "--help", "-h":
		fmt.Println(usage)
		return nil
	}
	return fmt.Errorf("unknown command %q\n\n%s", verb, usage)
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func runMigrate() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := signalContext()
	defer cancel()
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := store.Migrate(ctx, db); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	log.WithField("db", cfg.DBPath).Info("migrations applied")
	return nil
}
