## dump

`bungen dump` writes a consistent subset of a database as a psql data script,
for example the boot snapshot of an e2e test container.

```shell
bungen dump -c "$DB_URL" --config dump.yaml -o 02-data.sql
```

It selects the seed rows, expands each seed to its children up to
`child_depth`, then adds every row the selection references, recursively, so
every foreign key resolves. Only seed rows expand to children. All queries run
in one read-only repeatable-read snapshot and use `FROM ONLY`, so inheritance
children are separate tables. Tables owned by an extension are skipped. Row
counts per table go to stderr.

```yaml
seeds:
  # A query returns the ctid of the seed rows; the seed table is aliased t.
  - table: dossier
    query: >-
      SELECT DISTINCT ON (drs.dossier_status_id) t.ctid
      FROM ONLY dossier t JOIN dossier_rel_status drs ON drs.dossier_id = t.id
      ORDER BY drs.dossier_status_id, t.id DESC
  - table: mandat
    where: "date_insert >= now() - interval '1 year'"
    order_by: "id DESC"     # default: primary key descending
    limit: 20
    child_depth: 1          # default 1; 0 = parents only
    all_children: [acq_rel_dossier]  # these child tables are not capped for this seed
  - table: dossier_status   # no where, limit or query: the whole table
children_per_parent: 20     # children fetched per parent row and FK; default 20
exclude:                    # never entered by child expansion; still dumped as parents
  - "*.*_tracking_*"
  - phoning_history
attach:                     # "owned" child tables: all their rows referencing any
  - acq_recherche           # selected row come along, wherever that row comes from
  - acq_recherche_rel_geographic_zone
```

`attach` never selects other rows of an attached table, and attached rows do
not expand into non-attached children. A table both attached and excluded is
an error.

SQL in the config is inserted verbatim. A `query` on an inheritance parent
must use `FROM ONLY` itself.

The script is one transaction of `COPY ... FROM stdin` blocks followed by a
`setval` per sequence. It sets `session_replication_role = replica`, which
turns off triggers and FK checks while loading, so it must be loaded by a
superuser into a database that already has the schema, for example:

```go
postgres.WithInitScripts("01-schema.sql", "02-data.sql")
```

The output is deterministic: dumping unchanged data twice gives the same file.
