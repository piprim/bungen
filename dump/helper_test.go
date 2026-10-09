package dump

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LdDl/bungen/internal/testdb"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

const (
	fixtureSchema = "testdata/dump_schema.sql"
	fixtureData   = "testdata/dump_data.sql"
)

func openDB(dsn string) *bun.DB {
	return bun.NewDB(sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn))), pgdialect.New())
}

// newDB creates an empty database on the test server, loads files into it and
// returns it with its URL. The database is dropped when t ends.
func newDB(t *testing.T, files ...string) (*bun.DB, string) {
	t.Helper()
	ctx := context.Background()
	server := testdb.DSN(t)
	name := fmt.Sprintf("dump_%d", time.Now().UnixNano())

	admin := openDB(server)
	_, err := admin.DB.ExecContext(ctx, "CREATE DATABASE "+name)
	_ = admin.Close()
	if err != nil {
		if os.Getenv(testdb.EnvRequireDB) != "" {
			t.Fatalf("create test database: %v", err)
		}
		t.Skipf("cannot create a test database: %v", err)
	}
	t.Cleanup(func() {
		admin := openDB(server)
		defer func() { _ = admin.Close() }()
		if _, err := admin.DB.ExecContext(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			t.Logf("drop test database %s: %v", name, err)
		}
	})

	u, err := url.Parse(server)
	if err != nil {
		t.Fatalf("parse test DSN: %v", err)
	}
	u.Path = "/" + name
	dsn := u.String()

	db := openDB(dsn)
	t.Cleanup(func() { _ = db.Close() })
	for _, file := range files {
		script, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB.ExecContext(ctx, string(script)); err != nil {
			t.Fatalf("load %s: %v", file, err)
		}
	}
	return db, dsn
}

func mustConfig(t *testing.T, text string) *Config {
	t.Helper()
	cfg, err := ParseConfig(strings.NewReader(text))
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	return cfg
}

func readCatalog(t *testing.T, db *bun.DB) *catalog {
	t.Helper()
	ctx := context.Background()
	var cat *catalog
	err := withSnapshot(ctx, db, func(conn bun.Conn) error {
		var err error
		cat, err = loadCatalog(ctx, conn.Conn)
		return err
	})
	if err != nil {
		t.Fatalf("loadCatalog: %v", err)
	}
	return cat
}
