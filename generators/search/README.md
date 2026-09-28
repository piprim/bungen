## Search model generator

**Basic model required, which can be generated with one of the model generators**

Use `search` sub-command to execute generator:

`bungen search -h`

First create your database and tables in it

```sql
create extension if not exists "uuid-ossp";

create table "project"
(
    "project_id" uuid not null default uuid_generate_v4(),
    "code"       uuid,
    "name"       text not null,

    primary key ("project_id")
);

create table "user"
(
    "user_id"    serial      not null,
    "email"      varchar(64) not null,
    "activated"  bool        not null default false,
    "name"       varchar(128),
    "country_id" integer,
    "avatar"     bytea       not null,
    "avatar_alt" bytea,
    "api_keys"   bytea[],
    "logged_at"  timestamp,

    primary key ("user_id")
);

create schema "geo";

create table geo."country"
(
    "country_id" integer generated always as identity,
    "code"       varchar(3) not null,
    "coords"     integer[],

    primary key ("country_id")
);

alter table "user"
    add constraint "fk_user_country"
        foreign key ("country_id")
            references geo."country" ("country_id") on update restrict on delete restrict;
```

### Run generator

`bungen search -c postgres://user:password@localhost:5432/yourdb -o ~/output/model.go -t public.* -f`

You should get following search structs on model package:

```go
//nolint:all
//lint:file-ignore U1000 ignore unused code, it's generated
package readme

import (
	"time"

	"github.com/uptrace/bun"
)

const condition = "?.? = ?"

// base filters
type applier func(query bun.QueryBuilder) (bun.QueryBuilder, error)

type search struct {
	appliers []applier
}

func (s *search) apply(query bun.QueryBuilder) {
	for _, applier := range s.appliers {
		applier(query)
	}
}

func (s *search) where(query bun.QueryBuilder, table, field string, value interface{}) {

	query.Where(condition, bun.Ident(table), bun.Ident(field), value)

}

func (s *search) WithApply(a applier) {
	if s.appliers == nil {
		s.appliers = []applier{}
	}
	s.appliers = append(s.appliers, a)
}

func (s *search) With(condition string, params ...interface{}) {
	s.WithApply(func(query bun.QueryBuilder) (bun.QueryBuilder, error) {
		return query.Where(condition, params...), nil
	})
}

// Searcher is interface for every generated filter
type Searcher interface {
	Apply(query bun.QueryBuilder) bun.QueryBuilder
	Q() applier

	With(condition string, params ...interface{})
	WithApply(a applier)
}

type ProjectSearch struct {
	search

	ID   *string
	Code *string
	Name *string
}

func (s *ProjectSearch) Apply(query bun.QueryBuilder) bun.QueryBuilder {
	if s.ID != nil {
		s.where(query, Tables.Project.Alias, Columns.Project.ID, s.ID)
	}
	if s.Code != nil {
		s.where(query, Tables.Project.Alias, Columns.Project.Code, s.Code)
	}
	if s.Name != nil {
		s.where(query, Tables.Project.Alias, Columns.Project.Name, s.Name)
	}

	s.apply(query)

	return query
}

func (s *ProjectSearch) Q() applier {
	return func(query bun.QueryBuilder) (bun.QueryBuilder, error) {
		return s.Apply(query), nil
	}
}

type UserSearch struct {
	search

	ID        *int
	Email     *string
	Activated *bool
	Name      *string
	CountryID *int
	Avatar    *[]byte
	AvatarAlt *[]byte
	LoggedAt  *time.Time
}

func (s *UserSearch) Apply(query bun.QueryBuilder) bun.QueryBuilder {
	if s.ID != nil {
		s.where(query, Tables.User.Alias, Columns.User.ID, s.ID)
	}
	if s.Email != nil {
		s.where(query, Tables.User.Alias, Columns.User.Email, s.Email)
	}
	if s.Activated != nil {
		s.where(query, Tables.User.Alias, Columns.User.Activated, s.Activated)
	}
	if s.Name != nil {
		s.where(query, Tables.User.Alias, Columns.User.Name, s.Name)
	}
	if s.CountryID != nil {
		s.where(query, Tables.User.Alias, Columns.User.CountryID, s.CountryID)
	}
	if s.Avatar != nil {
		s.where(query, Tables.User.Alias, Columns.User.Avatar, s.Avatar)
	}
	if s.AvatarAlt != nil {
		s.where(query, Tables.User.Alias, Columns.User.AvatarAlt, s.AvatarAlt)
	}
	if s.LoggedAt != nil {
		s.where(query, Tables.User.Alias, Columns.User.LoggedAt, s.LoggedAt)
	}

	s.apply(query)

	return query
}

func (s *UserSearch) Q() applier {
	return func(query bun.QueryBuilder) (bun.QueryBuilder, error) {
		return s.Apply(query), nil
	}
}

type GeoCountrySearch struct {
	search

	ID   *int
	Code *string
}

func (s *GeoCountrySearch) Apply(query bun.QueryBuilder) bun.QueryBuilder {
	if s.ID != nil {
		s.where(query, Tables.GeoCountry.Alias, Columns.GeoCountry.ID, s.ID)
	}
	if s.Code != nil {
		s.where(query, Tables.GeoCountry.Alias, Columns.GeoCountry.Code, s.Code)
	}

	s.apply(query)

	return query
}

func (s *GeoCountrySearch) Q() applier {
	return func(query bun.QueryBuilder) (bun.QueryBuilder, error) {
		return s.Apply(query), nil
	}
}
```

### Try it

```go
package model

import (
	"fmt"
	"testing"

	"github.com/uptrace/bun"
)

func TestModel(t *testing.T) {
	// connecting to db
	pgdb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN("postgres://user:password@localhost:5432/yourdb")))
	db := bun.NewDB(pgdb, pgdialect.New(), bun.WithDiscardUnknownColumns())

	if _, err := db.Exec(`truncate table "user"; truncate table geo.country cascade;`); err != nil {
		panic(err)
	}

	// objects to insert
	toInsert := []GeoCountry{
		GeoCountry{
			Code:   "us",
			Coords: []int{1, 2},
		},
		GeoCountry{
			Code:   "uk",
			Coords: nil,
		},
	}

	// inserting
	ctx := context.Background()
	if _, err := db.NewInsert().Model(&toInsert).Column("code", "coords").Exec(ctx); err != nil {
		panic(err)
	}

    code := "us"
    country := GeoCountry{}
    search := GeoCountrySearch{
        Code: &code,
    }
    m := db.NewSelect().Model(&country)
	m = search.Apply(m.QueryBuilder()).Unwrap().(*bun.SelectQuery)

    ctx = context.Background()
	if err := m.Scan(ctx); err != nil {
		panic(err)
	}

	fmt.Printf("%#v\n", country)
}

```
