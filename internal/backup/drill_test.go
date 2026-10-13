package backup

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/octieght18/forge/internal/store"
	"github.com/octieght18/forge/internal/testsupport"
)

func TestScopeIsNotAnOffsiteEncryptedBackup(t *testing.T) {
	scope := Record()
	if scope.Encrypted || scope.Offsite {
		t.Fatal(scope)
	}
	joined := strings.Join(scope.Unrecoverable, " ")
	for _, phrase := range []string{"Temporal workflow history", "in-process evidence", "in-process audit events"} {
		if !strings.Contains(joined, phrase) {
			t.Fatal(joined)
		}
	}
}

func TestDisposableDumpRestoresTheWorkload(t *testing.T) {
	if os.Getenv("FORGE_TEST_ADMIN_DATABASE_URL") == "" {
		t.Skip("set FORGE_TEST_ADMIN_DATABASE_URL for the disposable backup drill")
	}
	db := testsupport.NewDatabase(t)
	issuer := testsupport.NewIdentity(t)
	ctx := context.Background()
	workload, err := db.Repo.CreateWorkload(ctx, store.Principal{Issuer: issuer.Server.URL, Subject: "ahmad", Role: "developer"}, []byte(`{"name":"backup-drill","description":"synthetic drill"}`))
	if err != nil {
		t.Fatal(err)
	}
	var source string
	if err := db.Migration.QueryRow(ctx, "SELECT current_database()").Scan(&source); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(source, "forge_http_") || strings.Contains(source, "forge-native") || strings.Contains(source, "forge-kubernetes") {
		t.Fatal("refusing a non-disposable database name")
	}
	password := adminPassword(t)
	dump := filepath.Join(t.TempDir(), "dump.sql")
	dumpStarted := time.Now()
	if err := runTool(t, password, "pg_dump", "--no-owner", "--no-acl", "-h", "127.0.0.1", "-U", "postgres", "-d", source, "-f", dump); err != nil {
		t.Fatal(err)
	}
	dumpDuration := time.Since(dumpStarted)
	if err := os.Chmod(dump, 0o600); err != nil {
		t.Fatal(err)
	}
	restoreName := source + "_r"
	quoted := pgx.Identifier{restoreName}.Sanitize()
	if _, err := db.Admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+quoted+" WITH (FORCE)")
	})
	restoreStarted := time.Now()
	if err := runTool(t, password, "psql", "-v", "ON_ERROR_STOP=1", "-h", "127.0.0.1", "-U", "postgres", "-d", restoreName, "-f", dump); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgx.ParseConfig(os.Getenv("FORGE_TEST_ADMIN_DATABASE_URL"))
	if err != nil {
		t.Fatal("restore configuration invalid")
	}
	cfg.Database = restoreName
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal("restore connection failed")
	}
	t.Cleanup(func() { conn.Close(ctx) })
	var id, name, description string
	var count int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM forge.workloads").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, "SELECT id::text, name, description FROM forge.workloads").Scan(&id, &name, &description); err != nil {
		t.Fatal(err)
	}
	restoreDuration := time.Since(restoreStarted)
	if count != 1 || id != workload.ID || name != "backup-drill" || description != "synthetic drill" {
		t.Fatalf("count=%d id=%s name=%s description=%s", count, id, name, description)
	}
	t.Logf("dump=%s restore=%s", dumpDuration, restoreDuration)
}

func adminPassword(t *testing.T) string {
	t.Helper()
	parsed, err := url.Parse(os.Getenv("FORGE_TEST_ADMIN_DATABASE_URL"))
	if err != nil {
		t.Fatal("admin configuration invalid")
	}
	password, ok := parsed.User.Password()
	if !ok || password == "" {
		t.Fatal("admin configuration invalid")
	}
	return password
}

func runTool(t *testing.T, password, name string, args ...string) error {
	t.Helper()
	command := exec.Command(name, args...)
	command.Env = append(os.Environ(), "PGPASSWORD="+password)
	output, err := command.CombinedOutput()
	if err != nil {
		text := string(output)
		if password != "" {
			text = strings.ReplaceAll(text, password, "redacted")
		}
		t.Fatalf("%s failed: %s", name, text)
	}
	return nil
}
