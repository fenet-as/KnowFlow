package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/fenet-as/KnowFlow/internal/config"
	"github.com/fenet-as/KnowFlow/internal/platform"
)

func main() {
	var dir string
	var dbURL string

	flag.StringVar(&dir, "dir", "migrations", "directory containing SQL migration files")
	flag.StringVar(&dbURL, "db", "", "PostgreSQL database connection URL (defaults to DATABASE_URL env var)")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	command := args[0]

	// If dbURL is not passed explicitly via flag, read from config/env
	if dbURL == "" {
		cfg, err := config.Load()
		if err == nil && cfg.DatabaseURL != "" {
			dbURL = cfg.DatabaseURL
		} else {
			dbURL = os.Getenv("DATABASE_URL")
		}
	}

	if dbURL == "" {
		log.Fatal("DATABASE_URL is required to run migrations")
	}

	fmt.Printf("Running migration command '%s' with directory '%s'...\n", command, dir)

	var err error
	switch command {
	case "up":
		err = platform.MigrateUp(dbURL, dir)
	case "down":
		err = platform.MigrateDown(dbURL, dir)
	case "reset":
		err = platform.MigrateReset(dbURL, dir)
	case "status":
		err = platform.MigrateStatus(dbURL, dir)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		log.Fatalf("Migration command failed: %v", err)
	}

	fmt.Println("Migration command completed successfully.")
}

func printUsage() {
	fmt.Println("Usage: go run ./cmd/migrate [flags] <command>")
	fmt.Println("Commands:")
	fmt.Println("  up      Apply all pending migrations")
	fmt.Println("  down    Roll back the last migration")
	fmt.Println("  reset   Roll back all migrations")
	fmt.Println("  status  Print status of all migrations")
	fmt.Println("Flags:")
	fmt.Println("  -dir    Path to migrations directory (default 'migrations')")
	fmt.Println("  -db     Postgres database URL (default $DATABASE_URL)")
}

