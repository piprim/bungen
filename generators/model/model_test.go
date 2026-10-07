package model

import (
	"reflect"
	"testing"

	"github.com/LdDl/bungen/generators/base"
	"github.com/LdDl/bungen/model"
)

func TestNewTemplateColumn_Tag(t *testing.T) {
	col := func(name, pgType string, nullable, pk bool, def string) model.Column {
		c := model.NewColumn(name, pgType, nullable, false, false, 0, pk, false, 0, nil, nil)
		c.Default = def
		return c
	}

	tests := []struct {
		name    string
		column  model.Column
		options Options
		want    string
	}{
		{
			name:   "serial primary key gets pk and autoincrement",
			column: col("user_id", model.TypePGInt4, false, true, `nextval('user_user_id_seq'::regclass)`),
			want:   "`bun:\"user_id,pk,autoincrement\"`",
		},
		{
			name:   "uuid primary key with function default",
			column: col("project_id", model.TypePGUuid, false, true, "uuid_generate_v4()"),
			want:   "`bun:\"project_id,pk,type:uuid,default:uuid_generate_v4()\"`",
		},
		{
			name:   "not null column without default gets notnull",
			column: col("email", model.TypePGVarchar, false, false, ""),
			want:   "`bun:\"email,notnull\"`",
		},
		{
			name:   "not null column with default gets notnull and default",
			column: col("activated", model.TypePGBool, false, false, "false"),
			want:   "`bun:\"activated,notnull,default:false\"`",
		},
		{
			name:   "nullable column gets only its name",
			column: col("name", model.TypePGVarchar, true, false, ""),
			want:   "`bun:\"name\"`",
		},
		{
			name:   "nullable column with default keeps default",
			column: col("created_at", model.TypePGTimestamptz, true, false, "now()"),
			want:   "`bun:\"created_at,default:now()\"`",
		},
		{
			name:   "default with a comma outside parentheses is dropped",
			column: col("tags", model.TypePGText, false, false, "'a,b'::text"),
			want:   "`bun:\"tags,notnull\"`",
		},
		{
			name:   "default with a double quote is dropped",
			column: col("label", model.TypePGText, false, false, `'say "hi"'::text`),
			want:   "`bun:\"label,notnull\"`",
		},
		{
			name: "identity column gets autoincrement and identity",
			column: func() model.Column {
				c := col("country_id", model.TypePGInt4, false, true, "")
				c.IsIdentity = true
				return c
			}(),
			want: "`bun:\"country_id,pk,autoincrement,identity\"`",
		},
		{
			name:   "array column keeps array tag",
			column: model.NewColumn("coords", model.TypePGInt4, true, false, true, 1, false, false, 0, nil, nil),
			want:   "`bun:\"coords,array\"`",
		},
		{
			name:    "soft delete column",
			column:  col("deleted_at", model.TypePGTimestamptz, true, false, ""),
			options: Options{SoftDelete: "deleted_at"},
			want:    "`bun:\"deleted_at,soft_delete\"`",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.options.KeepPK = true
			got := string(NewTemplateColumn(model.Entity{}, tt.column, tt.options).Tag)
			if got != tt.want {
				t.Errorf("tag = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestNewTemplateEntity_Tag(t *testing.T) {
	entity := model.NewEntity("geo", "country", nil, nil)

	t.Run("table tag uses table option with alias", func(t *testing.T) {
		got := string(NewTemplateEntity(entity, Options{}).Tag)
		want := "`bun:\"table:geo.country,alias:t\"`"
		if got != want {
			t.Errorf("tag = %s, want %s", got, want)
		}
	})

	t.Run("no alias option drops alias", func(t *testing.T) {
		got := string(NewTemplateEntity(entity, Options{NoAlias: true}).Tag)
		want := "`bun:\"table:geo.country\"`"
		if got != want {
			t.Errorf("tag = %s, want %s", got, want)
		}
	})
}

func TestNewTemplatePackage_Imports(t *testing.T) {
	has := func(imports []string, want string) bool {
		for _, imp := range imports {
			if imp == want {
				return true
			}
		}
		return false
	}

	t.Run("always imports bun", func(t *testing.T) {
		pkg := NewTemplatePackage(nil, Options{})
		if !pkg.HasImports || !has(pkg.Imports, "github.com/uptrace/bun") {
			t.Errorf("imports = %v, want github.com/uptrace/bun", pkg.Imports)
		}
	})

	t.Run("imports context only with ORM helpers", func(t *testing.T) {
		if pkg := NewTemplatePackage(nil, Options{}); has(pkg.Imports, "context") {
			t.Errorf("imports = %v, want no context", pkg.Imports)
		}
		if pkg := NewTemplatePackage(nil, Options{Options: base.Options{GenORM: true}}); !has(pkg.Imports, "context") {
			t.Errorf("imports = %v, want context", pkg.Imports)
		}
	})
}

func TestTagNameOption(t *testing.T) {
	options := Options{TagName: "boa"}

	t.Run("table tag uses the given key", func(t *testing.T) {
		entity := model.NewEntity("geo", "country", nil, nil)

		got := string(NewTemplateEntity(entity, options).Tag)
		want := "`boa:\"table:geo.country,alias:t\"`"
		if got != want {
			t.Errorf("tag = %s, want %s", got, want)
		}
	})

	t.Run("column tag uses the given key", func(t *testing.T) {
		column := model.NewColumn("email", model.TypePGText, false, false, false, 0, false, false, 0, nil, nil)

		got := string(NewTemplateColumn(model.Entity{}, column, options).Tag)
		want := "`boa:\"email,notnull\"`"
		if got != want {
			t.Errorf("tag = %s, want %s", got, want)
		}
	})

	t.Run("relation tag uses the given key", func(t *testing.T) {
		relation := model.NewRelation([]string{"country_id"}, "geo", "country", []string{"id"})

		got := string(NewTemplateRelation(relation, options).Tag)
		want := "`boa:\"join:country_id,rel:belongs-to\"`"
		if got != want {
			t.Errorf("tag = %s, want %s", got, want)
		}
	})

	t.Run("relation tag with join uses the given key", func(t *testing.T) {
		relation := model.NewRelation([]string{"country_id"}, "geo", "country", []string{"id"})

		got := string(NewTemplateRelationWithJoin(relation, "country_id", "id", options).Tag)
		want := "`boa:\"join:country_id=id,rel:belongs-to\"`"
		if got != want {
			t.Errorf("tag = %s, want %s", got, want)
		}
	})
}

func TestNewTemplateColumn_PresenceJSON(t *testing.T) {
	entity := model.NewEntity("public", "dossier", nil, nil)
	nullable := model.NewColumn("prefs", model.TypePGJSONB, true, true, false, 0, false, false, 0, nil, nil)
	notNull := model.NewColumn("payload", model.TypePGJSONB, false, true, false, 0, false, false, 0, nil, nil)

	t.Run("without override the type is presence.Of[json.RawMessage]", func(t *testing.T) {
		c := NewTemplateColumn(entity, nullable, Options{Presence: true})
		if c.Type != "presence.Of[json.RawMessage]" {
			t.Errorf("Type = %s", c.Type)
		}
	})

	t.Run("an override of a nullable column is wrapped in presence.Of", func(t *testing.T) {
		c := NewTemplateColumn(entity, nullable, Options{Presence: true, JSONTypes: map[string]string{"dossier.prefs": "Prefs"}})
		if c.Type != "presence.Of[Prefs]" {
			t.Errorf("Type = %s", c.Type)
		}
	})

	t.Run("an override drops the encoding/json import and keeps presence", func(t *testing.T) {
		c := NewTemplateColumn(entity, nullable, Options{Presence: true, JSONTypes: map[string]string{"dossier.prefs": "Prefs"}})
		if !reflect.DeepEqual(c.Imports, []string{model.PresenceImport}) {
			t.Errorf("Imports = %v", c.Imports)
		}
	})

	t.Run("an override of a not null column is used as is", func(t *testing.T) {
		c := NewTemplateColumn(entity, notNull, Options{Presence: true, JSONTypes: map[string]string{"dossier.payload": "Payload"}})
		if c.Type != "Payload" {
			t.Errorf("Type = %s", c.Type)
		}
		if len(c.Imports) != 0 {
			t.Errorf("Imports = %v", c.Imports)
		}
	})

	t.Run("without presence an override is used as is", func(t *testing.T) {
		c := NewTemplateColumn(entity, nullable, Options{JSONTypes: map[string]string{"dossier.prefs": "Prefs"}})
		if c.Type != "Prefs" {
			t.Errorf("Type = %s", c.Type)
		}
	})
}

func TestNewTemplatePackage_ImportsFollowOverrides(t *testing.T) {
	col := model.NewColumn("prefs", model.TypePGJSONB, true, true, false, 0, false, false, 0, nil, nil)
	entity := model.NewEntity("public", "dossier", []model.Column{col}, nil)

	t.Run("encoding/json is not imported when every json column is overridden", func(t *testing.T) {
		pkg := NewTemplatePackage([]model.Entity{entity}, Options{Presence: true, JSONTypes: map[string]string{"*": "Prefs"}})
		for _, imp := range pkg.Imports {
			if imp == "encoding/json" {
				t.Errorf("Imports = %v, encoding/json is unused", pkg.Imports)
			}
		}
	})

	t.Run("presence is imported", func(t *testing.T) {
		pkg := NewTemplatePackage([]model.Entity{entity}, Options{Presence: true})
		found := false
		for _, imp := range pkg.Imports {
			found = found || imp == model.PresenceImport
		}
		if !found {
			t.Errorf("Imports = %v, want %s", pkg.Imports, model.PresenceImport)
		}
	})
}
