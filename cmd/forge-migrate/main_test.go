package main

import (
	"strings"
	"testing"
)

func TestMigrationConfigurationErrorsAreSafe(t *testing.T) {
	t.Setenv("FORGE_MIGRATION_DATABASE_URL", "")
	if err := run(); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatal("missing connection accepted")
	}
	t.Setenv("FORGE_MIGRATION_DATABASE_URL", "postgres://fixture:private-fixture@127.0.0.1:0/fixture?sslmode=invalid")
	err := run()
	if err == nil || strings.Contains(err.Error(), "private-fixture") || strings.Contains(err.Error(), "postgres://") {
		t.Fatal("unsafe connection diagnostic")
	}
}
