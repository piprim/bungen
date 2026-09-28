package model

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/LdDl/bungen/model"
	"github.com/LdDl/bungen/util"
)

// TemplatePackage stores package info
type TemplatePackage struct {
	Package string

	HasImports bool
	Imports    []string

	Entities []TemplateEntity

	ORMNeeded   bool
	ORMDbStruct string
}

// NewTemplatePackage creates a package for template
func NewTemplatePackage(entities []model.Entity, options Options) TemplatePackage {
	imports := util.NewSet()
	imports.Add("github.com/uptrace/bun")
	if options.GenORM {
		imports.Add("context")
	}

	models := make([]TemplateEntity, len(entities))
	for i, entity := range entities {
		for _, imp := range entity.Imports {
			imports.Add(imp)
		}

		models[i] = NewTemplateEntity(entity, options)
	}

	return TemplatePackage{
		Package: options.Package,

		HasImports: imports.Len() > 0,
		Imports:    imports.Elements(),

		Entities:    models,
		ORMNeeded:   options.GenORM,
		ORMDbStruct: options.DBWrapName,
	}
}

// TemplateEntity stores struct info
type TemplateEntity struct {
	model.Entity

	Tag template.HTML

	NoAlias bool
	Alias   string

	Columns []TemplateColumn

	HasRelations bool
	Relations    []TemplateRelation
}

// NewTemplateEntity creates an entity for template
func NewTemplateEntity(entity model.Entity, options Options) TemplateEntity {
	if entity.HasMultiplePKs() {
		options.KeepPK = true
	}

	columns := make([]TemplateColumn, len(entity.Columns))
	for i, column := range entity.Columns {
		columns[i] = NewTemplateColumn(entity, column, options)
	}

	relations := make([]TemplateRelation, 0, len(entity.Relations))
	for _, column := range entity.Columns {
		if column.IsFK && column.Relation != nil && column.Relation.Relation != nil {
			relations = append(relations, NewTemplateRelationWithJoin(*column.Relation.Relation, column.PGName, column.Relation.RelationPK, options))
		}
	}

	// Old way: I'm not sure about this
	// relations := make([]TemplateRelation, len(entity.Relations))
	// for i, relation := range entity.Relations {
	// relations[i] = NewTemplateRelation(relation, options)
	// }

	tagName := tagName(options)
	tags := util.NewAnnotation()
	tags.AddTag(tagName, "table:"+entity.PGFullName)

	if !options.NoAlias {
		tags.AddTag(tagName, fmt.Sprintf("alias:%s", util.DefaultAlias))
	}

	if !options.NoDiscard {
		// Tag below now working with bun (it's global now)
		// tags.AddTag("bun", "discard_unknown_columns")
	}

	return TemplateEntity{
		Entity: entity,
		Tag:    template.HTML(fmt.Sprintf("`%s`", tags.String())),

		NoAlias: options.NoAlias,
		Alias:   util.DefaultAlias,

		Columns: columns,

		HasRelations: len(relations) > 0,
		Relations:    relations,
	}
}

// TemplateColumn stores column info
type TemplateColumn struct {
	model.Column

	Tag     template.HTML
	Comment template.HTML
}

// NewTemplateColumn creates a column for template
func NewTemplateColumn(entity model.Entity, column model.Column, options Options) TemplateColumn {
	if !options.KeepPK && column.IsPK {
		column.GoName = util.ID
	}

	if column.PGType == model.TypePGJSON || column.PGType == model.TypePGJSONB {
		if typ, ok := jsonType(options.JSONTypes, entity.PGSchema, entity.PGName, column.PGName); ok {
			column.Type = typ
		}
	}

	comment := ""
	tagName := tagName(options)
	tags := util.NewAnnotation()
	tags.AddTag(tagName, column.PGName)

	// pk tag
	if column.IsPK {
		tags.AddTag(tagName, "pk")
	}

	// types tag
	if column.PGType == model.TypePGHstore {
		tags.AddTag(tagName, "hstore")
	} else if column.IsArray {
		tags.AddTag(tagName, "array")
	}
	if column.PGType == model.TypePGUuid {
		tags.AddTag(tagName, "type:uuid")
	}

	// nullable tag: bun inserts zero values as is, so NOT NULL columns only need notnull
	if !column.Nullable && !column.IsPK {
		tags.AddTag(tagName, "notnull")
	}

	// default tag: autoincrement makes bun send DEFAULT for a zero value, which
	// serial and identity columns both need; identity additionally documents
	// the column as GENERATED AS IDENTITY. Any other default is passed through
	// when the struct tag can carry it.
	switch {
	case column.IsIdentity:
		tags.AddTag(tagName, "autoincrement")
		tags.AddTag(tagName, "identity")
	case strings.HasPrefix(column.Default, "nextval("):
		tags.AddTag(tagName, "autoincrement")
	case column.Default != "" && tagSafeDefault(column.Default):
		tags.AddTag(tagName, "default:"+column.Default)
	}

	// soft_delete tag
	if options.SoftDelete == column.PGName && column.Nullable && column.GoType == model.TypeTime && !column.IsArray {
		tags.AddTag(tagName, "soft_delete")
	}

	// ignore tag
	if column.GoType == model.TypeInterface {
		comment = "// unsupported"
		tags = util.NewAnnotation().AddTag(tagName, "-")
	}

	// add json tag
	if options.AddJSONTag {
		tags.AddTag("json", util.Underscore(column.PGName))
	}

	return TemplateColumn{
		Column: column,

		Tag:     template.HTML(fmt.Sprintf("`%s`", tags.String())),
		Comment: template.HTML(comment),
	}
}

// TemplateRelation stores relation info
type TemplateRelation struct {
	model.Relation

	Tag     template.HTML
	Comment template.HTML
}

// NewTemplateRelation creates relation for template
func NewTemplateRelation(relation model.Relation, options Options) TemplateRelation {
	comment := ""
	tagName := tagName(options)
	tags := util.NewAnnotation().AddTag("bun", "join:"+strings.Join(relation.FKFields, ","))
	tags.AddTag("bun", "rel:belongs-to")

	if len(relation.FKFields) > 1 {
		comment = "// unsupported"
		tags.AddTag(tagName, "-")
	}

	// add json tag
	if options.AddJSONTag {
		tags.AddTag("json", util.Underscore(relation.GoName))
	}

	return TemplateRelation{
		Relation: relation,

		Tag:     template.HTML(fmt.Sprintf("`%s`", tags.String())),
		Comment: template.HTML(comment),
	}
}

// NewTemplateRelationWithJoin creates relation for template with `join` tag component
// relPK - primary key in foreign table
func NewTemplateRelationWithJoin(relation model.Relation, relFK, relPK string, options Options) TemplateRelation {
	comment := ""
	tagName := tagName(options)
	tags := util.NewAnnotation().AddTag("bun", fmt.Sprintf("join:%s=%s", relFK, relPK))
	tags.AddTag("bun", "rel:belongs-to")

	if len(relation.FKFields) > 1 {
		comment = "// unsupported"
		tags.AddTag(tagName, "-")
	}

	// add json tag
	if options.AddJSONTag {
		tags.AddTag("json", util.Underscore(relation.GoName))
	}

	return TemplateRelation{
		Relation: relation,

		Tag:     template.HTML(fmt.Sprintf("`%s`", tags.String())),
		Comment: template.HTML(comment),
	}
}

func jsonType(mp map[string]string, schema, table, field string) (string, bool) {
	if mp == nil {
		return "", false
	}

	patterns := [][3]string{
		{schema, table, field},
		{schema, "*", field},
		{schema, table, "*"},
		{schema, "*", "*"},
	}

	var names []string
	for _, parts := range patterns {
		names = append(names, fmt.Sprintf("%s.%s", util.Join(parts[0], parts[1]), parts[2]))
		names = append(names, fmt.Sprintf("%s.%s", util.JoinF(parts[0], parts[1]), parts[2]))
	}
	names = append(names, util.Join(schema, table), "*")

	for _, name := range names {
		if v, ok := mp[name]; ok {
			return v, true
		}
	}

	return "", false
}

func tagName(options Options) string {
	return "bun"
}

// tagSafeDefault reports whether a DEFAULT expression survives a struct tag:
// a double quote ends the tag literal and a comma outside parentheses ends
// the bun option
func tagSafeDefault(def string) bool {
	depth := 0
	for _, r := range def {
		switch r {
		case '"':
			return false
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				return false
			}
		}
	}
	return true
}
