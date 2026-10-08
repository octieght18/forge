package environment_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/octieght18/forge/internal/environment"
	"github.com/octieght18/forge/internal/store"
	"github.com/octieght18/forge/internal/testsupport"
)

func TestReadOnlyIdentityRole(t *testing.T) {
	db := testsupport.NewDatabase(t)
	ctx := context.Background()
	principal := store.Principal{Issuer: "https://issuer.example/realms/forge", Subject: "owner-a", Role: "developer"}
	w, err := db.Repo.CreateWorkload(ctx, principal, []byte(`{"name":"environment-role-test","description":"synthetic"}`))
	if err != nil {
		t.Fatal(err)
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	secret := hex.EncodeToString(b[:])
	role := "forge_he_" + secret
	quoted := pgx.Identifier{role}.Sanitize()
	if _, err := db.Admin.Exec(ctx, "CREATE ROLE "+quoted+" LOGIN PASSWORD '"+secret+"'"); err != nil {
		t.Fatal(err)
	}
	cfg := db.Admin.Config().Copy()
	cfg.Database = db.Migration.Config().Database
	admin, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	var pool *pgxpool.Pool
	t.Cleanup(func() {
		if pool != nil {
			pool.Close()
		}
		if _, err := admin.Exec(ctx, "DROP OWNED BY "+quoted); err != nil {
			t.Error("role privileges cleanup failed")
		}
		admin.Close(ctx)
		if _, err := db.Admin.Exec(ctx, "DROP ROLE "+quoted); err != nil {
			t.Error("role cleanup failed")
		}
	})
	grants, err := os.ReadFile("../../deploy/postgres/environment-grants.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ReplaceAll(string(grants), "forge_environment", quoted)
	sql = strings.ReplaceAll(sql, "DATABASE forge ", "DATABASE "+pgx.Identifier{cfg.Database}.Sanitize()+" ")
	if _, err := admin.Exec(ctx, sql); err != nil {
		t.Fatal(err)
	}
	pconfig := db.Pool.Config().Copy()
	pconfig.ConnConfig.User = role
	pconfig.ConnConfig.Password = secret
	pool, err = pgxpool.NewWithConfig(ctx, pconfig)
	if err != nil {
		t.Fatal(err)
	}
	d := environment.Database{Pool: pool}
	id := environment.Identity{WorkloadID: w.ID, Issuer: principal.Issuer, Subject: principal.Subject}
	if ok, err := d.Verify(ctx, id); err != nil || !ok {
		t.Fatal("registered owner lookup failed", err)
	}
	id.Subject = "owner-b"
	if ok, err := d.Verify(ctx, id); err != nil || ok {
		t.Fatal("wrong owner accepted", err)
	}
	id.Subject = principal.Subject
	id.Issuer = "https://another.example/realms/forge"
	if ok, err := d.Verify(ctx, id); err != nil || ok {
		t.Fatal("wrong issuer accepted", err)
	}
	for _, sql := range []string{"SELECT name FROM forge.workloads", "SELECT * FROM forge.versions", "SELECT * FROM forge.runs", "SELECT * FROM forge.commands", "SELECT * FROM forge.schema_migrations", "UPDATE forge.workloads SET name='changed'", "DELETE FROM forge.workloads", "CREATE TABLE forge.forbidden(id integer)"} {
		if _, err := pool.Exec(ctx, sql); err == nil {
			t.Fatalf("role permitted %s", sql)
		}
	}
}
