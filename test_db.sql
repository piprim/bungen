drop schema if exists "public" cascade;
drop schema if exists "geo" cascade;

create schema "public";

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
