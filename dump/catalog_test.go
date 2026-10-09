package dump

import (
	"slices"
	"testing"
)

func TestLoadCatalog(t *testing.T) {
	db, _ := newDB(t, fixtureSchema)
	cat := readCatalog(t, db)

	t.Run("lists every table, sorted", func(t *testing.T) {
		want := []string{
			"public.Select", "public.a", "public.a_status", "public.audit", "public.b", "public.hub",
			"public.hub_log", "public.node", "public.person", "public.person_acq", "public.person_emp", "public.status",
		}
		if !slices.Equal(cat.Keys, want) {
			t.Errorf("Keys = %v, want %v", cat.Keys, want)
		}
	})

	t.Run("columns are in ordinal order", func(t *testing.T) {
		want := []string{"id", "b_id", "hub_id", "emp_id", "ts"}
		if got := cat.Tables["public.a"].Columns; !slices.Equal(got, want) {
			t.Errorf("a columns = %v, want %v", got, want)
		}
	})

	t.Run("an inheritance child has its own primary key", func(t *testing.T) {
		if got := cat.Tables["public.person_emp"].PK; !slices.Equal(got, []string{"id"}) {
			t.Errorf("person_emp PK = %v, want [id]", got)
		}
	})

	t.Run("a table without a primary key has an empty PK", func(t *testing.T) {
		if got := cat.Tables["public.a_status"].PK; len(got) != 0 {
			t.Errorf("a_status PK = %v, want none", got)
		}
	})

	t.Run("a FK can target a UNIQUE column", func(t *testing.T) {
		i := slices.IndexFunc(cat.FKs, func(fk foreignKey) bool {
			return fk.Child == "public.a_status" && fk.Parent == "public.status"
		})
		if i < 0 {
			t.Fatal("no FK a_status -> status")
		}
		fk := cat.FKs[i]
		if !slices.Equal(fk.ChildCols, []string{"status_code"}) || !slices.Equal(fk.ParentCols, []string{"code"}) {
			t.Errorf("FK columns = %v -> %v, want [status_code] -> [code]", fk.ChildCols, fk.ParentCols)
		}
	})

	t.Run("children lists the tables referencing a table", func(t *testing.T) {
		var got []string
		for _, fk := range cat.children("public.a") {
			got = append(got, fk.Child)
		}
		want := []string{"public.Select", "public.a_status", "public.b", "public.node"}
		if !slices.Equal(got, want) {
			t.Errorf("children(a) = %v, want %v", got, want)
		}
	})

	t.Run("parents lists the tables a table references", func(t *testing.T) {
		var got []string
		for _, fk := range cat.parents("public.a") {
			got = append(got, fk.Parent)
		}
		want := []string{"public.b", "public.hub", "public.person_emp"}
		if !slices.Equal(got, want) {
			t.Errorf("parents(a) = %v, want %v", got, want)
		}
	})

	t.Run("a sequence shared by inheritance children lists every table", func(t *testing.T) {
		i := slices.IndexFunc(cat.Sequences, func(s sequence) bool { return s.Name == "public.person_id_seq" })
		if i < 0 {
			t.Fatalf("no public.person_id_seq in %v", cat.Sequences)
		}
		want := []sequenceUse{{"public.person", "id"}, {"public.person_acq", "id"}, {"public.person_emp", "id"}}
		if got := cat.Sequences[i].Uses; !slices.Equal(got, want) {
			t.Errorf("uses = %v, want %v", got, want)
		}
	})
}
