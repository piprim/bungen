package model

import (
	"reflect"
	"testing"
)

func TestColumn_GoName(t *testing.T) {
	tests := []struct {
		name   string
		pgName string
		want   string
	}{
		{
			name:   "Should generate from simple word",
			pgName: "title",
			want:   "Title",
		},
		{
			name:   "Should generate from underscored",
			pgName: "short_title",
			want:   "ShortTitle",
		},
		{
			name:   "Should generate from camelCased",
			pgName: "shortTitle",
			want:   "ShortTitle",
		},
		{
			name:   "Should generate with underscored_id",
			pgName: "location_id",
			want:   "LocationID",
		},
		{
			name:   "Should generate with camelCasedId",
			pgName: "locationId",
			want:   "LocationID",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewColumn(tt.pgName, TypePGText, false, false, false, 0, false, false, 0, []string{}, CustomTypeMapping{})
			if c.GoName != tt.want {
				t.Errorf("Column.Name = %v, want %v", c.GoName, tt.want)
			}
		})
	}
}

func TestColumn_GoType(t *testing.T) {
	type fields struct {
		pgType   string
		array    bool
		dims     int
		nullable bool
		presence bool
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "Should generate int2 type",
			fields: fields{
				pgType:   TypePGInt2,
				array:    false,
				dims:     0,
				nullable: false,
			},
			want: "int",
		},
		{
			name: "Should generate int2 array type",
			fields: fields{
				pgType:   TypePGInt2,
				array:    true,
				dims:     2,
				nullable: false,
			},
			want: "[][]int",
		},
		{
			name: "Should generate int2 nullable type",
			fields: fields{
				pgType:   TypePGInt2,
				array:    true,
				dims:     2,
				nullable: true,
			},
			want: "[][]int",
		},
		{
			name: "Should generate struct type",
			fields: fields{
				pgType:   TypePGTimetz,
				array:    false,
				dims:     0,
				nullable: true,
			},
			want: "*time.Time",
		},
		{
			name: "Should generate interface for unknown type",
			fields: fields{
				pgType:   "unknown",
				array:    false,
				dims:     0,
				nullable: true,
			},
			want: "interface{}",
		},
		{
			name:   "presence: nullable int2 becomes presence.Of[int]",
			fields: fields{pgType: TypePGInt2, nullable: true, presence: true},
			want:   "presence.Of[int]",
		},
		{
			name:   "presence: nullable timestamptz becomes presence.Of[time.Time]",
			fields: fields{pgType: TypePGTimestamptz, nullable: true, presence: true},
			want:   "presence.Of[time.Time]",
		},
		{
			name:   "presence: not null text stays string",
			fields: fields{pgType: TypePGText, nullable: false, presence: true},
			want:   "string",
		},
		{
			name:   "presence: nullable jsonb becomes presence.Of[json.RawMessage]",
			fields: fields{pgType: TypePGJSONB, nullable: true, presence: true},
			want:   "presence.Of[json.RawMessage]",
		},
		{
			name:   "presence: not null json becomes json.RawMessage",
			fields: fields{pgType: TypePGJSON, nullable: false, presence: true},
			want:   "json.RawMessage",
		},
		{
			name:   "presence: nullable array stays a slice",
			fields: fields{pgType: TypePGInt4, array: true, dims: 1, nullable: true, presence: true},
			want:   "[]int",
		},
		{
			name:   "presence: nullable hstore stays a map",
			fields: fields{pgType: TypePGHstore, nullable: true, presence: true},
			want:   "map[string]string",
		},
		{
			name:   "presence: nullable bytea stays a byte slice",
			fields: fields{pgType: TypePGBytea, nullable: true, presence: true},
			want:   "[]byte",
		},
		{
			name:   "presence: unknown type stays interface{}",
			fields: fields{pgType: "unknown", nullable: true, presence: true},
			want:   "interface{}",
		},
		{
			name:   "presence: nullable interval stays a pointer, presence has no interval codec",
			fields: fields{pgType: TypePGInterval, nullable: true, presence: true},
			want:   "*time.Duration",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewColumn("test", tt.fields.pgType, tt.fields.nullable, tt.fields.presence, tt.fields.array, tt.fields.dims, false, false, 0, []string{}, CustomTypeMapping{})
			if got := c.Type; got != tt.want {
				t.Errorf("Column.Type = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestColumn_Imports(t *testing.T) {
	tests := []struct {
		name     string
		pgType   string
		nullable bool
		presence bool
		custom   CustomTypeMapping
		want     []string
	}{
		{
			name:   "plain timestamp imports time",
			pgType: TypePGTimestamptz,
			want:   []string{"time"},
		},
		{
			name:   "plain text imports nothing",
			pgType: TypePGText,
			want:   nil,
		},
		{
			name:     "presence nullable timestamp imports time and presence",
			pgType:   TypePGTimestamptz,
			nullable: true,
			presence: true,
			want:     []string{"time", PresenceImport},
		},
		{
			name:     "presence nullable json imports encoding/json and presence",
			pgType:   TypePGJSONB,
			nullable: true,
			presence: true,
			want:     []string{"encoding/json", PresenceImport},
		},
		{
			name:     "presence not null json imports encoding/json only",
			pgType:   TypePGJSONB,
			presence: true,
			want:     []string{"encoding/json"},
		},
		{
			name:     "presence not null text imports nothing",
			pgType:   TypePGText,
			presence: true,
			want:     nil,
		},
		{
			name:     "presence nullable interval imports time only",
			pgType:   TypePGInterval,
			nullable: true,
			presence: true,
			want:     []string{"time"},
		},
		{
			name:     "presence nullable custom uuid imports the custom package and presence",
			pgType:   TypePGUuid,
			nullable: true,
			presence: true,
			custom:   CustomTypeMapping{TypePGUuid: {PGType: TypePGUuid, GoType: "uuid.UUID", GoImport: "github.com/google/uuid"}},
			want:     []string{"github.com/google/uuid", PresenceImport},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewColumn("test", tt.pgType, tt.nullable, tt.presence, false, 0, false, false, 0, nil, tt.custom)
			if !reflect.DeepEqual(c.Imports, tt.want) {
				t.Errorf("Column.Imports = %v, want %v", c.Imports, tt.want)
			}
		})
	}
}
