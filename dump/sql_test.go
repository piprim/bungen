package dump

import "testing"

var (
	sqlDossier = &table{Schema: "public", Name: "dossier", Columns: []string{"id", "etat"}, PK: []string{"id"}}
	sqlMandat  = &table{Schema: "public", Name: "mandat", Columns: []string{"id", "dossier_id"}, PK: []string{"id"}}
	sqlRel     = &table{Schema: "erp", Name: "Rel", Columns: []string{"a", "b"}} // no PK, needs quoting
	sqlFK      = foreignKey{Child: "public.mandat", Parent: "public.dossier", ChildCols: []string{"dossier_id"}, ParentCols: []string{"id"}}
)

func intp(n int) *int { return &n }

func TestQuoting(t *testing.T) {
	t.Run("quoteIdent doubles quotes", func(t *testing.T) {
		if got := quoteIdent(`a"b`); got != `"a""b"` {
			t.Errorf("got %s", got)
		}
	})

	t.Run("quoteLiteral doubles single quotes", func(t *testing.T) {
		if got := quoteLiteral(`it's`); got != `'it''s'` {
			t.Errorf("got %s", got)
		}
	})

	t.Run("qualified quotes schema and name", func(t *testing.T) {
		if got := sqlRel.qualified(); got != `"erp"."Rel"` {
			t.Errorf("got %s", got)
		}
	})

	t.Run("tidArray quotes each ctid", func(t *testing.T) {
		if got := tidArray([]string{"(0,1)", "(3,12)"}); got != `{"(0,1)","(3,12)"}` {
			t.Errorf("got %s", got)
		}
	})

	t.Run("tidArray of nothing is an empty array", func(t *testing.T) {
		if got := tidArray(nil); got != "{}" {
			t.Errorf("got %s", got)
		}
	})
}

func TestSeedSQL(t *testing.T) {
	tests := []struct {
		name string
		t    *table
		seed Seed
		want string
	}{
		{
			"whole table, newest first",
			sqlDossier, Seed{},
			`SELECT t.ctid FROM ONLY "public"."dossier" t ORDER BY t."id" DESC`,
		},
		{
			"where, order_by and limit",
			sqlDossier, Seed{Where: "etat = 1 OR etat = 2", OrderBy: "id", Limit: intp(5)},
			`SELECT t.ctid FROM ONLY "public"."dossier" t WHERE (etat = 1 OR etat = 2) ORDER BY id LIMIT 5`,
		},
		{
			"no primary key orders by ctid",
			sqlRel, Seed{Limit: intp(2)},
			`SELECT t.ctid FROM ONLY "erp"."Rel" t ORDER BY t.ctid DESC LIMIT 2`,
		},
		{
			"query is used as is",
			sqlDossier, Seed{Query: "SELECT ctid FROM ONLY dossier WHERE data ? 'k'"},
			"SELECT ctid FROM ONLY dossier WHERE data ? 'k'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := seedSQL(tt.t, tt.seed); got != tt.want {
				t.Errorf("seedSQL =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestWalkSQL(t *testing.T) {
	t.Run("childSQL limits per parent", func(t *testing.T) {
		want := `SELECT x.ctid FROM ONLY "public"."dossier" p CROSS JOIN LATERAL (SELECT c.ctid FROM ONLY "public"."mandat" c WHERE (c."dossier_id") = (p."id") ORDER BY c."id" DESC LIMIT 20) x WHERE p.ctid = ANY($1::tid[])`
		if got := childSQL(sqlDossier, sqlMandat, sqlFK, 20); got != want {
			t.Errorf("childSQL =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("childSQL without a limit takes every child", func(t *testing.T) {
		want := `SELECT x.ctid FROM ONLY "public"."dossier" p CROSS JOIN LATERAL (SELECT c.ctid FROM ONLY "public"."mandat" c WHERE (c."dossier_id") = (p."id") ORDER BY c."id" DESC) x WHERE p.ctid = ANY($1::tid[])`
		if got := childSQL(sqlDossier, sqlMandat, sqlFK, 0); got != want {
			t.Errorf("childSQL =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("parentSQL follows the FK upwards", func(t *testing.T) {
		want := `SELECT p.ctid FROM ONLY "public"."dossier" p WHERE (p."id") IN (SELECT c."dossier_id" FROM ONLY "public"."mandat" c WHERE c.ctid = ANY($1::tid[]))`
		if got := parentSQL(sqlDossier, sqlMandat, sqlFK); got != want {
			t.Errorf("parentSQL =\n%s\nwant\n%s", got, want)
		}
	})
}

func TestCopySQL(t *testing.T) {
	t.Run("copyHeader lists every column", func(t *testing.T) {
		if got := copyHeader(sqlRel); got != `COPY "erp"."Rel" ("a", "b") FROM stdin;` {
			t.Errorf("got %s", got)
		}
	})

	t.Run("copySQL orders by primary key", func(t *testing.T) {
		want := `COPY (SELECT t."id", t."etat" FROM ONLY "public"."dossier" t WHERE t.ctid = ANY($1::tid[]) ORDER BY t."id") TO STDOUT`
		if got := copySQL(sqlDossier); got != want {
			t.Errorf("copySQL =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("copySQL orders a table without PK by its row text", func(t *testing.T) {
		want := `COPY (SELECT t."a", t."b" FROM ONLY "erp"."Rel" t WHERE t.ctid = ANY($1::tid[]) ORDER BY ROW(t.*)::text) TO STDOUT`
		if got := copySQL(sqlRel); got != want {
			t.Errorf("copySQL =\n%s\nwant\n%s", got, want)
		}
	})
}
