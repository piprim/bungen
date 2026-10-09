package dump

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LdDl/bungen/internal/testdb"
	"github.com/uptrace/bun"
)

// requireLoader returns the psql binary, or skips t when psql is missing or
// the test role is not a superuser (the script sets session_replication_role).
func requireLoader(t *testing.T, db *bun.DB) string {
	t.Helper()
	skip := t.Skipf
	if os.Getenv(testdb.EnvRequireDB) != "" {
		skip = t.Fatalf
	}
	psql, err := exec.LookPath("psql")
	if err != nil {
		skip("psql not found: %v", err)
	}
	var super bool
	err = db.DB.QueryRowContext(context.Background(),
		"SELECT rolsuper FROM pg_roles WHERE rolname = current_user").Scan(&super)
	if err != nil {
		t.Fatal(err)
	}
	if !super {
		skip("the test role is not a superuser; the script sets session_replication_role")
	}
	return psql
}

// loadScript runs script with psql the way the Postgres image runs init scripts.
func loadScript(t *testing.T, psql, dsn, script string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "data.sql")
	if err := os.WriteFile(file, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(psql, "-X", "-q", "-v", "ON_ERROR_STOP=1", "-d", dsn, "-f", file).CombinedOutput()
	if err != nil {
		t.Fatalf("psql: %v\n%s", err, out)
	}
}

// tableText renders rows of tbl as sorted text lines: the rows whose ctid is
// in ctids, or every row when all is set.
func tableText(t *testing.T, db *bun.DB, tbl *table, ctids []string, all bool) string {
	t.Helper()
	query := fmt.Sprintf(`SELECT coalesce(string_agg(ROW(t.*)::text, E'\n' ORDER BY ROW(t.*)::text), '') FROM ONLY %s t`, tbl.qualified())
	var args []any
	if !all {
		query += " WHERE t.ctid = ANY($1::tid[])"
		args = append(args, tidArray(ctids))
	}
	var text string
	if err := db.DB.QueryRowContext(context.Background(), query, args...).Scan(&text); err != nil {
		t.Fatalf("table %s: %v", tbl.key(), err)
	}
	return text
}

func scalar[T any](t *testing.T, db *bun.DB, query string) T {
	t.Helper()
	var v T
	if err := db.DB.QueryRowContext(context.Background(), query).Scan(&v); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return v
}

const roundTripConfig = `
seeds:
  - table: a
    where: "id IN (2, 3)"
  - table: node
    where: "id = 1"
    child_depth: 2
  - table: person
  - table: status
`

func TestRoundTrip(t *testing.T) {
	ctx := context.Background()
	src, _ := newDB(t, fixtureSchema, fixtureData)
	psql := requireLoader(t, src)
	cfg := mustConfig(t, roundTripConfig)

	var first, second bytes.Buffer
	if err := Dump(ctx, src, cfg, "roundtrip.yaml", &first, io.Discard); err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if err := Dump(ctx, src, cfg, "roundtrip.yaml", &second, io.Discard); err != nil {
		t.Fatalf("Dump: %v", err)
	}

	dst, dstDSN := newDB(t, fixtureSchema)
	loadScript(t, psql, dstDSN, first.String())

	var cat *catalog
	var rows rowSet
	err := withSnapshot(ctx, src, func(conn bun.Conn) error {
		var err error
		if cat, err = loadCatalog(ctx, conn.Conn); err != nil {
			return err
		}
		rows, err = walk(ctx, conn.Conn, cat, cfg)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("dumping twice gives identical scripts", func(t *testing.T) {
		if !bytes.Equal(first.Bytes(), second.Bytes()) {
			t.Error("the two dumps differ")
		}
	})

	for _, key := range cat.Keys {
		t.Run("table "+key+" matches", func(t *testing.T) {
			want := tableText(t, src, cat.Tables[key], rows.sorted(key), false)
			got := tableText(t, dst, cat.Tables[key], nil, true)
			if got != want {
				t.Errorf("loaded rows =\n%s\nwant\n%s", got, want)
			}
		})
	}

	t.Run("every foreign key resolves", func(t *testing.T) {
		for _, fk := range cat.FKs {
			t.Run(fk.Child+" -> "+fk.Parent, func(t *testing.T) {
				var notNull []string
				for _, c := range fk.ChildCols {
					notNull = append(notNull, "c."+quoteIdent(c)+" IS NOT NULL")
				}
				query := fmt.Sprintf(
					"SELECT count(*) FROM ONLY %s c WHERE %s AND NOT EXISTS (SELECT 1 FROM ONLY %s p WHERE (%s) = (%s))",
					cat.Tables[fk.Child].qualified(), strings.Join(notNull, " AND "),
					cat.Tables[fk.Parent].qualified(), columnList("p", fk.ParentCols), columnList("c", fk.ChildCols))
				if n := scalar[int64](t, dst, query); n != 0 {
					t.Errorf("%d orphan rows", n)
				}
			})
		}
	})

	t.Run("triggers did not fire", func(t *testing.T) {
		if n := scalar[int64](t, dst, "SELECT count(*) FROM audit"); n != 0 {
			t.Errorf("audit has %d rows, want 0", n)
		}
	})

	t.Run("sequences continue past the loaded rows", func(t *testing.T) {
		for _, s := range cat.Sequences {
			if !usedBy(s, rows) {
				continue
			}
			t.Run(s.Name, func(t *testing.T) {
				query := fmt.Sprintf("SELECT nextval(%s) > %s", quoteLiteral(s.Name), maxExpr(s, cat.Tables))
				if !scalar[bool](t, dst, query) {
					t.Errorf("nextval(%s) does not exceed the loaded values", s.Name)
				}
			})
		}
	})

	t.Run("an empty selection still loads", func(t *testing.T) {
		var out bytes.Buffer
		empty := mustConfig(t, "seeds:\n  - table: a\n    where: \"false\"\n")
		if err := Dump(ctx, src, empty, "empty.yaml", &out, io.Discard); err != nil {
			t.Fatalf("Dump: %v", err)
		}
		if strings.Contains(out.String(), "COPY ") {
			t.Errorf("script has COPY blocks:\n%s", out.String())
		}
		_, emptyDSN := newDB(t, fixtureSchema)
		loadScript(t, psql, emptyDSN, out.String())
	})
}

func TestRun(t *testing.T) {
	_, dsn := newDB(t, fixtureSchema, fixtureData)
	dir := t.TempDir()
	good := filepath.Join(dir, "good.yaml")
	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(good, []byte("seeds:\n  - table: status\n    child_depth: 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("seeds:\n  - table: nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	t.Run("writes the script to the output file", func(t *testing.T) {
		out := filepath.Join(dir, "good.sql")
		if err := run(ctx, dsn, good, out, io.Discard, io.Discard); err != nil {
			t.Fatalf("run: %v", err)
		}
		script, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(script), "-- Code generated by bungen dump from "+good+".") {
			t.Errorf("script starts with %q", string(script[:min(len(script), 80)]))
		}
	})

	t.Run("a failed dump leaves no output file", func(t *testing.T) {
		out := filepath.Join(dir, "missing.sql")
		if err := run(ctx, dsn, bad, out, io.Discard, io.Discard); err == nil {
			t.Fatal("run succeeded with an unknown seed table")
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if slices.ContainsFunc(entries, func(e os.DirEntry) bool { return strings.HasPrefix(e.Name(), "missing.sql") }) {
			t.Errorf("found output files in %v", entries)
		}
	})

	t.Run("a failed dump keeps an existing output file", func(t *testing.T) {
		out := filepath.Join(dir, "kept.sql")
		if err := os.WriteFile(out, []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := run(ctx, dsn, bad, out, io.Discard, io.Discard); err == nil {
			t.Fatal("run succeeded with an unknown seed table")
		}
		if got, _ := os.ReadFile(out); string(got) != "old" {
			t.Errorf("output file = %q, want old", got)
		}
	})
}

func TestConnect(t *testing.T) {
	if testing.Short() {
		t.Skip("waits past pgdriver's 10 s default read timeout")
	}
	db := connect(testdb.DSN(t))
	defer func() { _ = db.Close() }()

	t.Run("a statement may run longer than pgdriver's default read timeout", func(t *testing.T) {
		if _, err := db.DB.ExecContext(context.Background(), "SELECT pg_sleep(10.5)"); err != nil {
			t.Fatalf("long statement: %v", err)
		}
	})
}
