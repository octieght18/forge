// forge-migrate applies the embedded forward-only Forge product migrations.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/octieght18/forge/internal/store"
)

func run() error {
	dsn := os.Getenv("FORGE_MIGRATION_DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("FORGE_MIGRATION_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("migration connection failed")
	}
	defer conn.Close(context.Background())
	if err := store.Migrate(ctx, conn); err != nil {
		// Driver errors can contain connection details or SQL. Keep CLI output safe.
		return fmt.Errorf("migration failed; inspect database migration history and operator diagnostics")
	}
	fmt.Println("Forge product migrations applied")
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
