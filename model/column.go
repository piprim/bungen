package model

import (
	"github.com/LdDl/bungen/util"
)

type columnRelWrap struct {
	*Relation
	RelationPK string
}

// Column stores information about column
type Column struct {
	GoName string
	PGName string

	Type string

	GoType string
	PGType string

	Nullable bool

	IsArray    bool
	Dimensions int

	IsPK bool
	IsFK bool

	// Default is the column's DEFAULT expression as reported by PostgreSQL, empty when none
	Default string
	// IsIdentity is true for GENERATED ... AS IDENTITY columns
	IsIdentity bool
	// Relation *Relation
	Relation *columnRelWrap

	// Imports are the packages the Type needs
	Imports []string

	MaxLen int
	Values []string
}

// NewColumn creates Column from Postgres info. With presence, nullable
// columns are presence.Of[T] and json columns json.RawMessage.
func NewColumn(pgName string, pgType string, nullable, presence, array bool, dims int, pk, fk bool, len int, values []string, customTypes CustomTypeMapping) Column {
	var (
		err error
		ok  bool
	)

	column := Column{
		PGName:     pgName,
		PGType:     pgType,
		Nullable:   nullable,
		IsArray:    array,
		Dimensions: dims,
		IsPK:       pk,
		IsFK:       fk,
		MaxLen:     len,
		Values:     values,
		GoName:     util.ColumnName(pgName),
	}

	if customTypes == nil {
		customTypes = CustomTypeMapping{}
	}

	if column.GoType, ok = customTypes.GoType(pgType); !ok || column.GoType == "" {
		if column.GoType, err = GoType(pgType); err != nil {
			column.GoType = TypeInterface
		}
	}

	imp, ok := customTypes.GoImport(pgType)
	if !ok {
		imp = GoImport(pgType)
	}
	if imp != "" {
		column.Imports = append(column.Imports, imp)
	}

	switch {
	case column.IsArray:
		column.Type, err = GoSlice(pgType, dims)
	case presence:
		var extra []string
		column.Type, extra = GoPresence(pgType, column.GoType, nullable)
		column.Imports = append(column.Imports, extra...)
	case column.Nullable:
		column.Type, err = GoNullable(pgType, customTypes)
	default:
		column.Type = column.GoType
	}

	if err != nil {
		column.Type = column.GoType
	}

	return column
}

// AddRelation adds relation to column. Should be used if FK
func (c *Column) AddRelation(relation *Relation, relPK string) {
	c.Relation = &columnRelWrap{
		Relation:   relation,
		RelationPK: relPK,
	}
}
