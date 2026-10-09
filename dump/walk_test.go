package dump

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/LdDl/bungen/util"
	"github.com/uptrace/bun"
)

// statusRow identifies an a_status row, which has no primary key.
const statusRow = `t.a_id || ':' || t.status_code`

func runWalk(t *testing.T, db *bun.DB, config string) rowSet {
	t.Helper()
	cfg := mustConfig(t, config)
	ctx := context.Background()
	var rows rowSet
	err := withSnapshot(ctx, db, func(conn bun.Conn) error {
		cat, err := loadCatalog(ctx, conn.Conn)
		if err != nil {
			return err
		}
		rows, err = walk(ctx, conn.Conn, cat, cfg)
		return err
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return rows
}

// values evaluates expr on the selected rows of the table key, sorted as text.
func values(t *testing.T, db *bun.DB, rows rowSet, key, expr string) []string {
	t.Helper()
	ctids := rows.sorted(key)
	if len(ctids) == 0 {
		return nil
	}
	schema, name := util.Split(key)
	query := fmt.Sprintf("SELECT (%s)::text FROM ONLY %s.%s t WHERE t.ctid = ANY($1::tid[]) ORDER BY 1",
		expr, quoteIdent(schema), quoteIdent(name))
	res, err := db.DB.QueryContext(context.Background(), query, tidArray(ctids))
	if err != nil {
		t.Fatalf("values %s: %v", key, err)
	}
	defer func() { _ = res.Close() }()
	var out []string
	for res.Next() {
		var v string
		if err := res.Scan(&v); err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	if err := res.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func assertValues(t *testing.T, db *bun.DB, rows rowSet, key, expr string, want ...string) {
	t.Helper()
	t.Run(key, func(t *testing.T) {
		if got := values(t, db, rows, key, expr); !slices.Equal(got, want) {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	})
}

func TestWalk(t *testing.T) {
	db, _ := newDB(t, fixtureSchema, fixtureData)

	t.Run("parent closure follows the a-b cycle and stops", func(t *testing.T) {
		rows := runWalk(t, db, `
seeds:
  - table: a
    where: "id IN (2, 3)"
    child_depth: 0
`)
		assertValues(t, db, rows, "public.a", "t.id", "2", "3")
		assertValues(t, db, rows, "public.b", "t.id", "2", "3")
		assertValues(t, db, rows, "public.hub", "t.id", "1", "3")
		assertValues(t, db, rows, "public.person_emp", "t.id", "3")
		assertValues(t, db, rows, "public.a_status", statusRow)
		assertValues(t, db, rows, "public.hub_log", "t.id")
	})

	t.Run("children of a seed, then their parents", func(t *testing.T) {
		rows := runWalk(t, db, `
seeds:
  - table: a
    where: "id = 3"
`)
		assertValues(t, db, rows, "public.a", "t.id", "3")
		assertValues(t, db, rows, "public.b", "t.id", "3")
		assertValues(t, db, rows, "public.a_status", statusRow, "3:new", "3:open", "3:won")
		assertValues(t, db, rows, "public.status", "t.id", "1", "2", "3") // FK to the UNIQUE status.code
		assertValues(t, db, rows, "public.hub", "t.id", "1")
		assertValues(t, db, rows, "public.hub_log", "t.id") // hub is a pulled-in parent: no children
	})

	t.Run("children_per_parent caps each parent, not the edge", func(t *testing.T) {
		rows := runWalk(t, db, `
seeds:
  - table: a
    where: "id IN (2, 3)"
    children_per_parent: 1
`)
		assertValues(t, db, rows, "public.a_status", statusRow, "2:open", "3:won")
	})

	for depth, want := range [][]string{{"1"}, {"1", "2"}, {"1", "2", "3"}} {
		t.Run(fmt.Sprintf("child_depth %d through the self-FK", depth), func(t *testing.T) {
			rows := runWalk(t, db, fmt.Sprintf(`
seeds:
  - table: node
    where: "id = 1"
    child_depth: %d
`, depth))
			assertValues(t, db, rows, "public.node", "t.id", want...)
		})
	}

	t.Run("exclude blocks child expansion", func(t *testing.T) {
		rows := runWalk(t, db, `
seeds:
  - table: a
    where: "id = 3"
exclude: [a_status]
`)
		assertValues(t, db, rows, "public.a_status", statusRow)
		assertValues(t, db, rows, "public.status", "t.id")
		assertValues(t, db, rows, "public.b", "t.id", "3")
	})

	t.Run("exclude never blocks parents or seeds", func(t *testing.T) {
		rows := runWalk(t, db, `
seeds:
  - table: node
    where: "id = 4"
    child_depth: 0
exclude: [a, node]
`)
		assertValues(t, db, rows, "public.node", "t.id", "1", "2", "3", "4")
		assertValues(t, db, rows, "public.a", "t.id", "1", "6")
	})

	t.Run("an inheritance parent does not pick up child rows", func(t *testing.T) {
		rows := runWalk(t, db, `
seeds:
  - table: person
    child_depth: 0
`)
		assertValues(t, db, rows, "public.person", "t.id", "1", "2")
		assertValues(t, db, rows, "public.person_emp", "t.id")
		assertValues(t, db, rows, "public.person_acq", "t.id")
	})

	t.Run("a query seed covers every group", func(t *testing.T) {
		rows := runWalk(t, db, `
seeds:
  - table: a
    child_depth: 0
    query: >-
      SELECT DISTINCT ON (s.status_code) t.ctid
      FROM ONLY a t JOIN a_status s ON s.a_id = t.id
      ORDER BY s.status_code, t.id DESC
`)
		assertValues(t, db, rows, "public.a", "t.id", "3", "5", "6")
	})

	t.Run("a seed without limit or query takes the whole table", func(t *testing.T) {
		rows := runWalk(t, db, `
seeds:
  - table: status
    child_depth: 0
`)
		assertValues(t, db, rows, "public.status", "t.id", "1", "2", "3", "4")
	})

	t.Run("where with the default order and a limit", func(t *testing.T) {
		rows := runWalk(t, db, `
seeds:
  - table: a
    where: "hub_id = 1"
    limit: 1
    child_depth: 0
`)
		assertValues(t, db, rows, "public.a", "t.id", "6")
	})

	t.Run("order_by and limit", func(t *testing.T) {
		rows := runWalk(t, db, `
seeds:
  - table: a
    order_by: "id"
    limit: 2
    child_depth: 0
`)
		assertValues(t, db, rows, "public.a", "t.id", "1", "2")
	})

	t.Run("overlapping seeds keep each row once", func(t *testing.T) {
		rows := runWalk(t, db, `
seeds:
  - table: a
    where: "id IN (2, 3)"
    child_depth: 0
  - table: a
    where: "id IN (3, 4)"
    child_depth: 0
`)
		assertValues(t, db, rows, "public.a", "t.id", "2", "3", "4")
	})

	t.Run("where with the jsonb ? operator", func(t *testing.T) {
		rows := runWalk(t, db, `
seeds:
  - table: a_status
    where: "note ? 'k'"
    child_depth: 0
`)
		assertValues(t, db, rows, "public.a_status", statusRow, "1:new")
	})

	t.Run("quoted table name", func(t *testing.T) {
		rows := runWalk(t, db, `
seeds:
  - table: a
    where: "id = 1"
`)
		assertValues(t, db, rows, "public.Select", "t.id", "1", "2")
	})

	t.Run("an unknown seed table is an error", func(t *testing.T) {
		cfg := mustConfig(t, "seeds:\n  - table: nope\n")
		ctx := context.Background()
		err := withSnapshot(ctx, db, func(conn bun.Conn) error {
			cat, err := loadCatalog(ctx, conn.Conn)
			if err != nil {
				return err
			}
			_, err = walk(ctx, conn.Conn, cat, cfg)
			return err
		})
		if err == nil || !strings.Contains(err.Error(), "public.nope not found") {
			t.Fatalf("err = %v, want public.nope not found", err)
		}
	})
}

func TestRowSetSummary(t *testing.T) {
	rows := rowSet{}
	rows.add("public.a", []string{"(0,1)"})
	rows.add("public.b", []string{"(0,1)", "(0,2)"})
	rows.add("public.c", nil)

	var b strings.Builder
	rows.summary(&b)

	t.Run("largest table first, empty tables left out", func(t *testing.T) {
		want := "       2  public.b\n       1  public.a\n"
		if b.String() != want {
			t.Errorf("summary =\n%q\nwant\n%q", b.String(), want)
		}
	})
}

func TestWalkAttach(t *testing.T) {
	db, _ := newDB(t, fixtureSchema, fixtureData)

	t.Run("all_children lifts the cap for the listed tables only", func(t *testing.T) {
		rows := runWalk(t, db, `
seeds:
  - table: a
    where: "id IN (1, 3)"
    children_per_parent: 1
    all_children: [a_status]
`)
		assertValues(t, db, rows, "public.a_status", statusRow, "1:new", "3:new", "3:open", "3:won")
		assertValues(t, db, rows, "public.Select", "t.id", "2") // a1 has two, the cap keeps one
	})

	t.Run("an attached table brings the rows referencing a pulled-in parent", func(t *testing.T) {
		rows := runWalk(t, db, `
seeds:
  - table: a
    where: "id = 3"
    child_depth: 0
attach:
  - table: hub_log
    to: hub
`)
		assertValues(t, db, rows, "public.hub", "t.id", "1")
		// Only hub 1's log rows (hub_id = 1 + g % 3 = 1), uncapped: all 10.
		assertValues(t, db, rows, "public.hub_log", "t.id", "12", "15", "18", "21", "24", "27", "3", "30", "6", "9")
		// hub 1's other children are not attached.
		assertValues(t, db, rows, "public.a", "t.id", "3")
		assertValues(t, db, rows, "public.person_emp", "t.id")
	})

	t.Run("attachment chains through attached rows", func(t *testing.T) {
		rows := runWalk(t, db, `
seeds:
  - table: node
    where: "id = 1"
    child_depth: 0
attach:
  - table: node
    to: node
`)
		assertValues(t, db, rows, "public.node", "t.id", "1", "2", "3", "4")
	})

	t.Run("a link table is attached through the named parent only", func(t *testing.T) {
		rows := runWalk(t, db, `
seeds:
  - table: a
    where: "id = 3"
    child_depth: 0
attach:
  - table: hub_tag
    to: hub
`)
		// hub 1 brings its tag (1, 1), whose status 1 comes in as a parent.
		// Status 1 must not bring hub 2's tag (2, 1).
		assertValues(t, db, rows, "public.hub_tag", "t.hub_id || ':' || t.status_id", "1:1")
		assertValues(t, db, rows, "public.hub", "t.id", "1")
	})

	t.Run("an attach rule without a matching FK is an error", func(t *testing.T) {
		cfg := mustConfig(t, "seeds:\n  - table: a\nattach:\n  - table: hub_log\n    to: status\n")
		ctx := context.Background()
		err := withSnapshot(ctx, db, func(conn bun.Conn) error {
			cat, err := loadCatalog(ctx, conn.Conn)
			if err != nil {
				return err
			}
			_, err = walk(ctx, conn.Conn, cat, cfg)
			return err
		})
		if err == nil || !strings.Contains(err.Error(), "no foreign key from public.hub_log to public.status") {
			t.Fatalf("err = %v, want no foreign key from public.hub_log to public.status", err)
		}
	})

	t.Run("a table both attached and excluded is an error", func(t *testing.T) {
		cfg := mustConfig(t, "seeds:\n  - table: a\nattach:\n  - table: hub_log\n    to: hub\nexclude: [\"hub_*\"]\n")
		ctx := context.Background()
		err := withSnapshot(ctx, db, func(conn bun.Conn) error {
			cat, err := loadCatalog(ctx, conn.Conn)
			if err != nil {
				return err
			}
			_, err = walk(ctx, conn.Conn, cat, cfg)
			return err
		})
		if err == nil || !strings.Contains(err.Error(), "public.hub_log is both attached and excluded") {
			t.Fatalf("err = %v, want public.hub_log is both attached and excluded", err)
		}
	})
}
