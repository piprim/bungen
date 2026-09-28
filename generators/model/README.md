## Basic model generator

Use `model` sub-command to execute generator:

`bungen model -h`

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

`bungen model -c postgres://user:password@localhost:5432/yourdb -o ~/output/model.go -t public.* -f`

You should get following models on model package:

```go
//nolint:all
//lint:file-ignore U1000 ignore unused code, it's generated
package readme

import (
	"github.com/uptrace/bun"
	"time"
)

var Columns = struct {
	Project struct {
		ID, Code, Name string
	}
	User struct {
		ID, Email, Activated, Name, CountryID, Avatar, AvatarAlt, ApiKeys, LoggedAt string

		Country string
	}
	GeoCountry struct {
		ID, Code, Coords string
	}
}{
	Project: struct {
		ID, Code, Name string
	}{
		ID:   "project_id",
		Code: "code",
		Name: "name",
	},
	User: struct {
		ID, Email, Activated, Name, CountryID, Avatar, AvatarAlt, ApiKeys, LoggedAt string

		Country string
	}{
		ID:        "user_id",
		Email:     "email",
		Activated: "activated",
		Name:      "name",
		CountryID: "country_id",
		Avatar:    "avatar",
		AvatarAlt: "avatar_alt",
		ApiKeys:   "api_keys",
		LoggedAt:  "logged_at",

		Country: "Country",
	},
	GeoCountry: struct {
		ID, Code, Coords string
	}{
		ID:     "country_id",
		Code:   "code",
		Coords: "coords",
	},
}

var Tables = struct {
	Project struct {
		Name, Alias string
	}
	User struct {
		Name, Alias string
	}
	GeoCountry struct {
		Name, Alias string
	}
}{
	Project: struct {
		Name, Alias string
	}{
		Name:  "project",
		Alias: "t",
	},
	User: struct {
		Name, Alias string
	}{
		Name:  "user",
		Alias: "t",
	},
	GeoCountry: struct {
		Name, Alias string
	}{
		Name:  "geo.country",
		Alias: "t",
	},
}

type Project struct {
	bun.BaseModel `bun:"table:project,alias:t"`

	ID   string  `bun:"project_id,pk,type:uuid,default:uuid_generate_v4()"`
	Code *string `bun:"code,type:uuid"`
	Name string  `bun:"name,notnull"`
}

type User struct {
	bun.BaseModel `bun:"table:user,alias:t"`

	ID        int        `bun:"user_id,pk,autoincrement"`
	Email     string     `bun:"email,notnull"`
	Activated bool       `bun:"activated,notnull,default:false"`
	Name      *string    `bun:"name"`
	CountryID *int       `bun:"country_id"`
	Avatar    []byte     `bun:"avatar,notnull"`
	AvatarAlt []byte     `bun:"avatar_alt"`
	ApiKeys   [][]byte   `bun:"api_keys,array"`
	LoggedAt  *time.Time `bun:"logged_at"`

	Country *GeoCountry `bun:"join:country_id=country_id,rel:belongs-to"`
}

type GeoCountry struct {
	bun.BaseModel `bun:"table:geo.country,alias:t"`

	ID     int    `bun:"country_id,pk,autoincrement,identity"`
	Code   string `bun:"code,notnull"`
	Coords []int  `bun:"coords,array"`
}

/* Common ORM queries */

// Just a wrapper around database connection
```

### Try it

```go
package model

import (
	"fmt"
	"testing"

	"github.com/uptrace/bun"
)

const AllColumns = "t.*"

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

	// selecting
	var toSelect []GeoCountry
	ctx = context.Background()
	if err := db.NewSelect().Model(&toSelect).Scan(ctx); err != nil {
		panic(err)
	}

	fmt.Printf("%#v\n", toSelect)

	// user with fk
	newUser := User{
		Email:     "test@gmail.com",
		Activated: true,
		CountryID: &toSelect[0].ID,
	}

	// inserting
	ctx = context.Background()
	if _, err := db.NewInsert().Model(&newUser).Column("email", "activated", "country_id").Exec(ctx); err != nil {
		panic(err)
	}

	// selecting inserted user
	user := User{}
	m := db.NewSelect().
		Column(AllColumns).
		Relation(Columns.User.Country).
		Where(`? = ?`, bun.Ident(Columns.User.Email), "test@gmail.com")
	
	ctx = context.Background()
	if err := m.Scan(ctx); err != nil {
		panic(err)
	}

	fmt.Printf("%#v\n", user)
	fmt.Printf("%#v\n", user.Country)
}

```
