package dump

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
)

// batchSize bounds the ctid array sent in one query.
const batchSize = 5000

// rowSet holds the selected rows: table key → set of ctids.
type rowSet map[string]map[string]struct{}

// add records ctids for table and returns the ones not seen before.
func (r rowSet) add(table string, ctids []string) []string {
	var added []string
	for _, c := range ctids {
		set := r[table]
		if set == nil {
			set = map[string]struct{}{}
			r[table] = set
		}
		if _, ok := set[c]; !ok {
			set[c] = struct{}{}
			added = append(added, c)
		}
	}
	return added
}

func (r rowSet) sorted(table string) []string {
	return slices.Sorted(maps.Keys(r[table]))
}

// summary writes the row count of every table, largest first.
func (r rowSet) summary(w io.Writer) {
	keys := slices.Collect(maps.Keys(r))
	slices.SortFunc(keys, func(a, b string) int {
		return cmp.Or(cmp.Compare(len(r[b]), len(r[a])), strings.Compare(a, b))
	})
	for _, k := range keys {
		_, _ = fmt.Fprintf(w, "%8d  %s\n", len(r[k]), k)
	}
}

// walk selects the seed rows, expands each seed to its children up to its
// depth, then adds every row the selection references, and the rows attached
// to it by an attach rule, until nothing is new. Only seed rows expand to
// children; a row is expanded at most once, by the first seed that reaches it.
func walk(ctx context.Context, conn *sql.Conn, cat *catalog, cfg *Config) (rowSet, error) {
	for _, s := range cfg.Seeds {
		if cat.Tables[s.key()] == nil {
			return nil, fmt.Errorf("dump: seed table %s not found (or owned by an extension)", s.key())
		}
	}
	for _, a := range cfg.Attach {
		child, parent := qualify(a.Table), qualify(a.To)
		if !slices.ContainsFunc(cat.FKs, func(fk foreignKey) bool { return fk.Child == child && fk.Parent == parent }) {
			return nil, fmt.Errorf("dump: attach: no foreign key from %s to %s", child, parent)
		}
		if cfg.excluded(child) {
			return nil, fmt.Errorf("dump: table %s is both attached and excluded", child)
		}
	}

	rows := rowSet{}
	for _, s := range cfg.Seeds {
		if err := expandSeed(ctx, conn, cat, cfg, s, rows); err != nil {
			return nil, err
		}
	}
	if err := closeSelection(ctx, conn, cat, cfg, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func expandSeed(ctx context.Context, conn *sql.Conn, cat *catalog, cfg *Config, s Seed, rows rowSet) error {
	key := s.key()
	ctids, err := queryStrings(ctx, conn, seedSQL(cat.Tables[key], s))
	if err != nil {
		return fmt.Errorf("dump: seed %s: %w", key, err)
	}

	frontier := map[string][]string{key: rows.add(key, ctids)}
	for range s.childDepth() {
		next := map[string][]string{}
		for _, parent := range slices.Sorted(maps.Keys(frontier)) {
			for _, fk := range cat.children(parent) {
				if cfg.excluded(fk.Child) {
					continue
				}
				query := childSQL(cat.Tables[fk.Parent], cat.Tables[fk.Child], fk, cfg.childLimit(s, fk.Child))
				found, err := queryBatched(ctx, conn, query, frontier[parent])
				if err != nil {
					return fmt.Errorf("dump: children %s of %s: %w", fk.Child, fk.Parent, err)
				}
				next[fk.Child] = append(next[fk.Child], rows.add(fk.Child, found)...)
			}
		}
		frontier = next
	}
	return nil
}

// closeSelection adds, for every selected row, the rows it references and
// all the rows attached to it by an attach rule, until nothing is new. Sets
// only grow and are finite, so FK cycles and self-FKs terminate.
func closeSelection(ctx context.Context, conn *sql.Conn, cat *catalog, cfg *Config, rows rowSet) error {
	work := map[string][]string{}
	for key := range rows {
		work[key] = rows.sorted(key)
	}
	for len(work) > 0 {
		next := map[string][]string{}
		add := func(table string, found []string) {
			if added := rows.add(table, found); len(added) > 0 {
				next[table] = append(next[table], added...)
			}
		}
		for _, key := range slices.Sorted(maps.Keys(work)) {
			for _, fk := range cat.parents(key) {
				query := parentSQL(cat.Tables[fk.Parent], cat.Tables[fk.Child], fk)
				found, err := queryBatched(ctx, conn, query, work[key])
				if err != nil {
					return fmt.Errorf("dump: parents %s of %s: %w", fk.Parent, fk.Child, err)
				}
				add(fk.Parent, found)
			}
			for _, fk := range cat.children(key) {
				if !cfg.attachedVia(fk) {
					continue
				}
				query := childSQL(cat.Tables[fk.Parent], cat.Tables[fk.Child], fk, 0)
				found, err := queryBatched(ctx, conn, query, work[key])
				if err != nil {
					return fmt.Errorf("dump: attached %s of %s: %w", fk.Child, fk.Parent, err)
				}
				add(fk.Child, found)
			}
		}
		work = next
	}
	return nil
}

// queryBatched runs query, which takes a tid array as $1, over ctids in
// batches.
func queryBatched(ctx context.Context, conn *sql.Conn, query string, ctids []string) ([]string, error) {
	var out []string
	for chunk := range slices.Chunk(ctids, batchSize) {
		found, err := queryStrings(ctx, conn, query, tidArray(chunk))
		if err != nil {
			return nil, err
		}
		out = append(out, found...)
	}
	return out, nil
}

// queryStrings returns the first column of every row. It goes through the
// driver directly, so a query without args reaches Postgres unchanged.
func queryStrings(ctx context.Context, conn *sql.Conn, query string, args ...any) ([]string, error) {
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
