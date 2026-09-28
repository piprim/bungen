## Validate functions generator

**Basic model required, which can be generated with one of the model generators**

Use `validation` sub-command to execute generator:

`bungen validation -h`

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

`bungen validation -c postgres://user:password@localhost:5432/yourdb -o ~/output/model.go -t public.* -f`

You should get following functions on model package:

```go
//nolint:all
//lint:file-ignore U1000 ignore unused code, it's generated
package readme

import (
	"unicode/utf8"
)

const (
	ErrEmptyValue = "empty"
	ErrMaxLength  = "len"
	ErrWrongValue = "value"
)

func (m User) Validate() (errors map[string]string, valid bool) {
	errors = map[string]string{}

	if utf8.RuneCountInString(m.Email) > 64 {
		errors[Columns.User.Email] = ErrMaxLength
	}

	if m.Name != nil && utf8.RuneCountInString(*m.Name) > 128 {
		errors[Columns.User.Name] = ErrMaxLength
	}

	if m.CountryID != nil && *m.CountryID == 0 {
		errors[Columns.User.CountryID] = ErrEmptyValue
	}

	return errors, len(errors) == 0
}

func (m GeoCountry) Validate() (errors map[string]string, valid bool) {
	errors = map[string]string{}

	if utf8.RuneCountInString(m.Code) > 3 {
		errors[Columns.GeoCountry.Code] = ErrMaxLength
	}

	return errors, len(errors) == 0
}
```

### Try it

```go
package model

import (
	"fmt"
	"testing"
)

func TestModel(t *testing.T) {
    code := "should fail on length"
    country := GeoCountry{
    	Code: code,
    }
    errors, valid := country.Validate()

	fmt.Printf("%#v\n", errors)
	fmt.Printf("%#v\n", valid)
}

```
