package dump

import (
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	cfg, err := ParseConfig(strings.NewReader(`
seeds:
  - table: dossier
    where: "etat = 1"
    order_by: "id DESC"
    limit: 20
  - table: erp.facture
    child_depth: 2
    children_per_parent: 5
  - table: dossier_status
children_per_parent: 7
exclude:
  - phoning_history
  - "*.*_tracking_*"
`))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	t.Run("reads every seed", func(t *testing.T) {
		if len(cfg.Seeds) != 3 {
			t.Fatalf("got %d seeds, want 3", len(cfg.Seeds))
		}
	})

	t.Run("seed key adds the public schema", func(t *testing.T) {
		if got := cfg.Seeds[0].key(); got != "public.dossier" {
			t.Errorf("key = %q, want public.dossier", got)
		}
	})

	t.Run("seed key keeps an explicit schema", func(t *testing.T) {
		if got := cfg.Seeds[1].key(); got != "erp.facture" {
			t.Errorf("key = %q, want erp.facture", got)
		}
	})

	t.Run("child depth defaults to 1", func(t *testing.T) {
		if got := cfg.Seeds[0].childDepth(); got != 1 {
			t.Errorf("childDepth = %d, want 1", got)
		}
	})

	t.Run("child depth can be set", func(t *testing.T) {
		if got := cfg.Seeds[1].childDepth(); got != 2 {
			t.Errorf("childDepth = %d, want 2", got)
		}
	})

	t.Run("children per parent falls back to the top level", func(t *testing.T) {
		if got := cfg.childrenPerParent(cfg.Seeds[0]); got != 7 {
			t.Errorf("childrenPerParent = %d, want 7", got)
		}
	})

	t.Run("children per parent can be set per seed", func(t *testing.T) {
		if got := cfg.childrenPerParent(cfg.Seeds[1]); got != 5 {
			t.Errorf("childrenPerParent = %d, want 5", got)
		}
	})

	t.Run("children per parent defaults to 20", func(t *testing.T) {
		if got := (&Config{}).childrenPerParent(Seed{}); got != 20 {
			t.Errorf("childrenPerParent = %d, want 20", got)
		}
	})

	t.Run("a bare exclude name matches the public table", func(t *testing.T) {
		if !cfg.excluded("public.phoning_history") {
			t.Error("public.phoning_history is not excluded")
		}
	})

	t.Run("a bare exclude name does not match other schemas", func(t *testing.T) {
		if cfg.excluded("erp.phoning_history") {
			t.Error("erp.phoning_history is excluded")
		}
	})

	t.Run("an exclude glob matches across schemas", func(t *testing.T) {
		if !cfg.excluded("erp.personne_tracking_connexion") {
			t.Error("erp.personne_tracking_connexion is not excluded")
		}
	})

	t.Run("other tables are not excluded", func(t *testing.T) {
		if cfg.excluded("public.dossier") {
			t.Error("public.dossier is excluded")
		}
	})
}

func TestParseConfigErrors(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"empty file", "", "empty"},
		{"no seeds", "exclude: [x]\n", "no seeds"},
		{"missing table", "seeds:\n  - where: \"x\"\n", "table is required"},
		{"query with where", "seeds:\n  - table: a\n    query: \"SELECT ctid FROM a\"\n    where: \"x\"\n", "query excludes"},
		{"query with limit", "seeds:\n  - table: a\n    query: \"SELECT ctid FROM a\"\n    limit: 1\n", "query excludes"},
		{"unknown field", "seeds:\n  - table: a\n    limt: 1\n", "limt"},
		{"zero limit", "seeds:\n  - table: a\n    limit: 0\n", "limit must be at least 1"},
		{"negative child depth", "seeds:\n  - table: a\n    child_depth: -1\n", "child_depth must be at least 0"},
		{"zero children per parent", "seeds:\n  - table: a\nchildren_per_parent: 0\n", "children_per_parent must be at least 1"},
		{"bad exclude pattern", "seeds:\n  - table: a\nexclude: [\"[\"]\n", "exclude"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig(strings.NewReader(tt.yaml))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestAttachConfig(t *testing.T) {
	cfg, err := ParseConfig(strings.NewReader(`
seeds:
  - table: dossier
    all_children: [acq_rel_dossier]
  - table: mandat
attach:
  - acq_recherche
  - "erp.*_zone"
`))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	t.Run("all_children lifts the cap for its seed", func(t *testing.T) {
		if got := cfg.childLimit(cfg.Seeds[0], "public.acq_rel_dossier"); got != 0 {
			t.Errorf("childLimit = %d, want 0 (no limit)", got)
		}
	})

	t.Run("other child tables keep the cap", func(t *testing.T) {
		if got := cfg.childLimit(cfg.Seeds[0], "public.mandat"); got != 20 {
			t.Errorf("childLimit = %d, want 20", got)
		}
	})

	t.Run("all_children does not apply to other seeds", func(t *testing.T) {
		if got := cfg.childLimit(cfg.Seeds[1], "public.acq_rel_dossier"); got != 20 {
			t.Errorf("childLimit = %d, want 20", got)
		}
	})

	t.Run("attach matches a bare public name", func(t *testing.T) {
		if !cfg.attached("public.acq_recherche") {
			t.Error("public.acq_recherche is not attached")
		}
	})

	t.Run("attach matches a glob", func(t *testing.T) {
		if !cfg.attached("erp.geographic_zone") {
			t.Error("erp.geographic_zone is not attached")
		}
	})

	t.Run("other tables are not attached", func(t *testing.T) {
		if cfg.attached("public.dossier") {
			t.Error("public.dossier is attached")
		}
	})

	t.Run("a bad attach pattern is an error", func(t *testing.T) {
		_, err := ParseConfig(strings.NewReader("seeds:\n  - table: a\nattach: [\"[\"]\n"))
		if err == nil || !strings.Contains(err.Error(), "attach") {
			t.Fatalf("err = %v, want an attach error", err)
		}
	})

	t.Run("a bad all_children pattern is an error", func(t *testing.T) {
		_, err := ParseConfig(strings.NewReader("seeds:\n  - table: a\n    all_children: [\"[\"]\n"))
		if err == nil || !strings.Contains(err.Error(), "all_children") {
			t.Fatalf("err = %v, want an all_children error", err)
		}
	})
}
