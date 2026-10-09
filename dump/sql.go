package dump

import (
	"fmt"
	"strconv"
	"strings"
)

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func quoteLiteral(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}

func (t *table) qualified() string {
	return quoteIdent(t.Schema) + "." + quoteIdent(t.Name)
}

// columnList renders cols, each prefixed with alias when alias is not empty.
func columnList(alias string, cols []string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = quoteIdent(c)
		if alias != "" {
			parts[i] = alias + "." + parts[i]
		}
	}
	return strings.Join(parts, ", ")
}

// tidArray renders ctids as a tid[] literal, for a $1::tid[] argument.
func tidArray(ctids []string) string {
	parts := make([]string, len(ctids))
	for i, c := range ctids {
		parts[i] = `"` + c + `"`
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// keyOrder orders newest first: by primary key descending, or by ctid
// descending without one.
func keyOrder(alias string, t *table) string {
	if len(t.PK) == 0 {
		return alias + ".ctid DESC"
	}
	parts := make([]string, len(t.PK))
	for i, c := range t.PK {
		parts[i] = alias + "." + quoteIdent(c) + " DESC"
	}
	return strings.Join(parts, ", ")
}

// stableOrder orders rows the same way on every dump: by primary key, or by
// the whole row as text without one.
func stableOrder(alias string, t *table) string {
	if len(t.PK) == 0 {
		return "ROW(" + alias + ".*)::text"
	}
	return columnList(alias, t.PK)
}

// seedSQL selects the ctids of a seed's rows.
func seedSQL(t *table, s Seed) string {
	if s.Query != "" {
		return s.Query
	}
	q := "SELECT t.ctid FROM ONLY " + t.qualified() + " t"
	if s.Where != "" {
		q += " WHERE (" + s.Where + ")"
	}
	order := s.OrderBy
	if order == "" {
		order = keyOrder("t", t)
	}
	q += " ORDER BY " + order
	if s.Limit != nil {
		q += " LIMIT " + strconv.Itoa(*s.Limit)
	}
	return q
}

// childSQL selects up to perParent rows of child referencing each parent row
// whose ctid is in $1; every such row when perParent is 0.
func childSQL(parent, child *table, fk foreignKey, perParent int) string {
	limit := ""
	if perParent > 0 {
		limit = " LIMIT " + strconv.Itoa(perParent)
	}
	return fmt.Sprintf(
		"SELECT x.ctid FROM ONLY %s p CROSS JOIN LATERAL (SELECT c.ctid FROM ONLY %s c WHERE (%s) = (%s) ORDER BY %s%s) x WHERE p.ctid = ANY($1::tid[])",
		parent.qualified(), child.qualified(),
		columnList("c", fk.ChildCols), columnList("p", fk.ParentCols),
		keyOrder("c", child), limit,
	)
}

// parentSQL selects the parent rows referenced by the child rows whose ctid is
// in $1.
func parentSQL(parent, child *table, fk foreignKey) string {
	return fmt.Sprintf(
		"SELECT p.ctid FROM ONLY %s p WHERE (%s) IN (SELECT %s FROM ONLY %s c WHERE c.ctid = ANY($1::tid[]))",
		parent.qualified(), columnList("p", fk.ParentCols),
		columnList("c", fk.ChildCols), child.qualified(),
	)
}

// copyHeader starts the COPY block that loads t.
func copyHeader(t *table) string {
	return fmt.Sprintf("COPY %s (%s) FROM stdin;", t.qualified(), columnList("", t.Columns))
}

// copySQL streams the rows of t whose ctid is in $1, in stable order.
func copySQL(t *table) string {
	return fmt.Sprintf(
		"COPY (SELECT %s FROM ONLY %s t WHERE t.ctid = ANY($1::tid[]) ORDER BY %s) TO STDOUT",
		columnList("t", t.Columns), t.qualified(), stableOrder("t", t),
	)
}
